package copilot

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/internal/safepaths"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"

	ghzip "github.com/cli/cli/v2/internal/zip"
)

type CopilotOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)

	Args   []string
	Remove bool
}

func NewCmdCopilot(f *cmdutil.Factory, runF func(*CopilotOptions) error) *cobra.Command {
	opts := &CopilotOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "copilot [flags]",
		Short: "Run the GitHub Copilot CLI",
		Long: heredoc.Docf(`
            Runs the GitHub Copilot CLI.

            If already installed, it will use the version found in your PATH.

            If not installed, it will be downloaded to %[2]s.

            Use %[1]s--remove%[1]s to remove the downloaded Copilot CLI.

			This command is supported on Linux, Darwin, and Windows.

            Learn more at https://gh.io/copilot-cli
        `, "`", copilotBinaryPath()),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
				return cmd.Help()
			}

			if len(args) > 1 && slices.Contains(args, "--remove") {
				return fmt.Errorf("cannot use --remove with args")
			}

			if len(args) > 0 && args[0] == "--remove" {
				// We do not captures args when --remove is provided.
				opts.Remove = true
			} else {
				opts.Args = args
			}

			if runF != nil {
				return runF(opts)
			}
			return runCopilot(opts)
		},
	}

	cmdutil.DisableAuthCheck(cmd)

	// We add this flag, even though flag parsing is disabled for this command
	// so the flag still appears in the help text.
	cmd.Flags().Bool("remove", false, "Remove the downloaded Copilot CLI")

	return cmd
}

func runCopilot(opts *CopilotOptions) error {
	if opts.Remove {
		if err := removeCopilot(); err != nil {
			return err
		}

		if opts.IO.IsStdoutTTY() {
			fmt.Fprintln(opts.IO.ErrOut, "Copilot CLI removed successfully")
		}
		return nil
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}

	copilotPath, err := ensureCopilot(httpClient, opts.IO)
	if err != nil {
		return err
	}

	externalCmd := exec.Command(copilotPath, opts.Args...)
	externalCmd.Stdin = opts.IO.In
	externalCmd.Stdout = opts.IO.Out
	externalCmd.Stderr = opts.IO.ErrOut

	if err := externalCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

const copilotBinaryName = "copilot"

func copilotBinaryPath() string {
	return filepath.Join(config.DataDir(), copilotBinaryName)
}

func ensureCopilot(httpClient *http.Client, io *iostreams.IOStreams) (string, error) {
	// First check if copilot is in PATH
	if path, err := exec.LookPath(copilotBinaryName); err == nil {
		return path, nil
	}

	// Check in gh's data directory
	installDir := copilotBinaryPath()
	binaryName := copilotBinaryName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	localPath := filepath.Join(installDir, binaryName)

	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	return downloadCopilot(httpClient, io, installDir, localPath)
}

func downloadCopilot(httpClient *http.Client, ios *iostreams.IOStreams, installDir, localPath string) (string, error) {
	platform := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	if arch != "x64" && arch != "arm64" {
		return "", fmt.Errorf("unsupported architecture: %s (supported: x64, arm64)", arch)
	}

	var archiveURL string
	var archiveName string
	var isZip bool
	switch platform {
	case "windows":
		archiveName = fmt.Sprintf("copilot-%s-%s.zip", platform, arch)
		archiveURL = fmt.Sprintf("https://github.com/github/copilot-cli/releases/latest/download/%s", archiveName)
		isZip = true
	case "linux", "darwin":
		archiveName = fmt.Sprintf("copilot-%s-%s.tar.gz", platform, arch)
		archiveURL = fmt.Sprintf("https://github.com/github/copilot-cli/releases/latest/download/%s", archiveName)
	default:
		return "", fmt.Errorf("unsupported platform: %s (supported: linux, darwin, windows)", platform)
	}

	checksumsURL := "https://github.com/github/copilot-cli/releases/latest/download/SHA256SUMS.txt"

	fmt.Fprintf(ios.ErrOut, "Downloading Copilot CLI from %s\n", archiveURL)

	expectedChecksum, err := fetchExpectedChecksum(httpClient, checksumsURL, archiveName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch checksums: %w", err)
	}

	resp, err := httpClient.Get(archiveURL)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Download to temp file while calculating checksum
	tmpFile, err := os.CreateTemp("", "copilot-download-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	hasher := sha256.New()
	if _, err := io.Copy(tmpFile, io.TeeReader(resp.Body, hasher)); err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}

	// Validate checksum
	actualChecksumHex := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksumHex != expectedChecksum {
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksumHex)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to seek temp file: %w", err)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create install directory: %w", err)
	}

	// Extract from the downloaded data
	if isZip {
		err = extractZip(tmpFile.Name(), installDir)
	} else {
		err = extractTarGz(tmpFile, installDir)
	}
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(localPath); err != nil {
		return "", fmt.Errorf("copilot binary unavailable: %w", err)
	}

	fmt.Fprintln(ios.ErrOut, "Copilot CLI installed successfully")
	return localPath, nil
}

// fetchExpectedChecksum downloads the SHA256SUMS.txt file and returns the expected checksum for the given archive name.
func fetchExpectedChecksum(client *http.Client, checksumsURL, archiveName string) (string, error) {
	resp, err := client.Get(checksumsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download checksums: %s", resp.Status)
	}

	// Parse the checksums file. Possible formats are:
	// - "<checksum>  <filename>" (two whitespaces)
	// - "<checksum> <filename>"
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			checksum := fields[0]
			filename := fields[len(fields)-1]
			if filename == archiveName {
				return checksum, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read checksums: %w", err)
	}

	return "", fmt.Errorf("checksum not found for %s", archiveName)
}

// extractZip reads a ZIP archive at path and extracts its contents into destDir.
// It returns an error if the archive cannot be read,
// or if any file or directory within the archive cannot be created or written.
func extractZip(path, destDir string) error {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	absPath, err := safepaths.ParseAbsolute(destDir)
	if err != nil {
		return err
	}

	// As of the time of writing, ghzip.ExtractZip will safely skip files that
	// would result in path traversal. This is an issue for our use-case because
	// we want to error out before extracting if there's any such file.
	// To avoid breaking the shared ghzip.ExtractZip code that expects unsafe
	// paths to be ignored and no error produced, we pre-validate here,
	// producing an error if any such file is found.
	for _, f := range zipReader.File {
		_, err := absPath.Join(f.Name)
		if err != nil {
			return err
		}
	}

	if err := ghzip.ExtractZip(&zipReader.Reader, absPath); err != nil {
		return err
	}

	return nil
}

// extractTarGz reads a TAR.GZ archive from r and extracts its contents into destDir.
// It returns an error if the archive cannot be read,
// or if any file or directory within the archive cannot be created or written.
func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		absPath, err := safepaths.ParseAbsolute(destDir)
		if err != nil {
			return err
		}
		absPath, err = absPath.Join(header.Name)
		if err != nil {
			return err
		}
		target := absPath.String()

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			if err := extractFile(target, os.FileMode(header.Mode)&0777, tr); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractFile creates a file at target with the given mode and copies content from r.
func extractFile(target string, mode os.FileMode, r io.Reader) (err error) {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if cerr := out.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("failed to close file: %w", cerr)
		}
	}()
	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil

}

func removeCopilot() error {
	installDir := copilotBinaryPath()
	return removeCopilotFromDir(installDir)
}

func removeCopilotFromDir(installDir string) error {
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("failed to remove Copilot CLI: %w", err)
	}
	return nil
}
