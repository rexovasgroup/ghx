package copilot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
)

func TestNewCmdCopilot(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: ios,
	}

	cmd := NewCmdCopilot(f)

	if cmd.Use != "copilot [flags]" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	if cmd.Short != "Run the GitHub Copilot CLI" {
		t.Errorf("unexpected Short: %s", cmd.Short)
	}

	if !cmd.DisableFlagParsing {
		t.Error("expected DisableFlagParsing to be true")
	}
}

func TestRemoveCopilot(t *testing.T) {
	t.Run("removes existing install directory", func(t *testing.T) {
		ios, _, _, stderr := iostreams.Test()

		// Create a temporary directory to simulate the install directory
		tmpDir := t.TempDir()
		installDir := filepath.Join(tmpDir, "copilot")
		if err := os.MkdirAll(installDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
		// Create a dummy file in the directory
		dummyFile := filepath.Join(installDir, "copilot")
		if err := os.WriteFile(dummyFile, []byte("test"), 0755); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := removeCopilotFromDir(ios, installDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if _, err := os.Stat(installDir); !os.IsNotExist(err) {
			t.Error("expected install directory to be removed")
		}

		if stderr.String() != "Copilot CLI removed successfully\n" {
			t.Errorf("unexpected stderr: %s", stderr.String())
		}
	})

	t.Run("handles non-existent directory", func(t *testing.T) {
		ios, _, _, stderr := iostreams.Test()

		tmpDir := t.TempDir()
		installDir := filepath.Join(tmpDir, "copilot")

		err := removeCopilotFromDir(ios, installDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if stderr.String() != "Copilot CLI is not installed\n" {
			t.Errorf("unexpected stderr: %s", stderr.String())
		}
	})
}
