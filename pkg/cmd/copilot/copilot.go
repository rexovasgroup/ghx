package copilot

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

const copilotBinaryName = "copilot"

// sanitizeArchivePath validates that the archive entry path is safe and returns
// the cleaned target path. It prevents Zip Slip attacks by ensuring the path
// doesn't escape the destination directory.
func sanitizeArchivePath(destDir, entryName string) (string, error) {
	target := filepath.Join(destDir, entryName)
	if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal file path in archive: %s", entryName)
	}
	return target, nil
}

func NewCmdCopilot(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copilot [flags]",
		Short: "Run the GitHub Copilot CLI",
		Long: heredoc.Docf(`
            Runs the GitHub Copilot CLI.
            

            If already installed, it will use the version found in your PATH.

            If not installed, it will be downloaded to %s.

            Use --remove to remove the downloaded Copilot CLI.

            Learn more at https://gh.io/copilot-cli
        `, filepath.Join(config.DataDir(), "copilot")),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "--remove" {
				return removeCopilot(f.IOStreams)
			}
			return runCopilot(f.HttpClient, f.IOStreams, args)
		},
	}

	cmdutil.DisableAuthCheck(cmd)

	return cmd
}

func runCopilot(httpClient func() (*http.Client, error), io *iostreams.IOStreams, args []string) error {
	copilotPath, err := ensureCopilot(httpClient, io)
	if err != nil {
		return err
	}

	externalCmd := exec.Command(copilotPath, args...)
	externalCmd.Stdin = io.In
	externalCmd.Stdout = io.Out
	externalCmd.Stderr = io.ErrOut

	if err := externalCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func ensureCopilot(httpClient func() (*http.Client, error), io *iostreams.IOStreams) (string, error) {
	// First check if copilot is in PATH
	if path, err := exec.LookPath(copilotBinaryName); err == nil {
		return path, nil
	}

	// Check in gh's data directory
	installDir := filepath.Join(config.DataDir(), "copilot")
	binaryName := copilotBinaryName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	localPath := filepath.Join(installDir, binaryName)

	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// Download copilot
	return downloadCopilot(httpClient, io, installDir, localPath)
}

func downloadCopilot(httpClient func() (*http.Client, error), io *iostreams.IOStreams, installDir, localPath string) (string, error) {
	platform := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	if arch != "x64" && arch != "arm64" {
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	var url string
	var isZip bool
	switch platform {
	case "windows":
		url = fmt.Sprintf("https://github.com/github/copilot-cli/releases/latest/download/copilot-%s-%s.zip", platform, arch)
		isZip = true
	case "linux", "darwin":
		url = fmt.Sprintf("https://github.com/github/copilot-cli/releases/latest/download/copilot-%s-%s.tar.gz", platform, arch)
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}

	fmt.Fprintf(io.ErrOut, "Downloading Copilot CLI from %s\n", url)

	client, err := httpClient()
	if err != nil {
		return "", err
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create install directory: %w", err)
	}

	if isZip {
		err = extractZip(resp.Body, installDir)
	} else {
		err = extractTarGz(resp.Body, installDir)
	}
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(localPath); err != nil {
		return "", fmt.Errorf("copilot binary not found after extraction")
	}

	fmt.Fprintln(io.ErrOut, "Copilot CLI installed successfully")
	return localPath, nil
}

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

		target, err := sanitizeArchivePath(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("failed to close file: %w", err)
			}
		}
	}
	return nil
}

func removeCopilot(io *iostreams.IOStreams) error {
	installDir := filepath.Join(config.DataDir(), "copilot")
	return removeCopilotFromDir(io, installDir)
}

func removeCopilotFromDir(io *iostreams.IOStreams, installDir string) error {
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		fmt.Fprintln(io.ErrOut, "Copilot CLI is not installed")
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("failed to remove Copilot CLI: %w", err)
	}
	fmt.Fprintln(io.ErrOut, "Copilot CLI removed successfully")
	return nil
}

func extractZip(r io.Reader, destDir string) error {
	// Create a temporary file to store the zip content
	tmpFile, err := os.CreateTemp("", "copilot-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		target, err := sanitizeArchivePath(destDir, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create file: %w", err)
		}

		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		if err := out.Close(); err != nil {
			rc.Close()
			return fmt.Errorf("failed to close file: %w", err)
		}
		rc.Close()
	}
	return nil
}
