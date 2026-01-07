package copilot

import (
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

	if cmd.Short != "GitHub Copilot CLI" {
		t.Errorf("unexpected Short: %s", cmd.Short)
	}

	if !cmd.DisableFlagParsing {
		t.Error("expected DisableFlagParsing to be true")
	}
}
