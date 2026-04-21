package telemetry_test

import (
	"errors"
	"testing"

	"github.com/cli/cli/v2/internal/gh"
	ghmock "github.com/cli/cli/v2/internal/gh/mock"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCmd builds a "gh <parent> <name>" command tree for testing. The returned
// command is the leaf that callers interact with.
func newCmd(t *testing.T, parentName, name string, withRepoFlag, withHostnameFlag bool) *cobra.Command {
	t.Helper()
	leaf := &cobra.Command{Use: name}
	if withRepoFlag {
		leaf.Flags().String("repo", "", "")
	}
	if withHostnameFlag {
		leaf.Flags().String("hostname", "", "")
	}
	root := &cobra.Command{Use: "gh"}
	if parentName != "" {
		parent := &cobra.Command{Use: parentName}
		root.AddCommand(parent)
		parent.AddCommand(leaf)
	} else {
		root.AddCommand(leaf)
	}
	return leaf
}

func TestGuessTargetHost(t *testing.T) {
	t.Run("returns host from --repo flag", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", true, false)
		require.NoError(t, cmd.Flags().Set("repo", "cli/cli"))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "github.com", got)
	})

	t.Run("returns host from --repo flag with custom host", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", true, false)
		require.NoError(t, cmd.Flags().Set("repo", "ghe.example.com/cli/cli"))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "ghe.example.com", got)
	})

	t.Run("returns host from --hostname flag", func(t *testing.T) {
		cmd := newCmd(t, "auth", "login", false, true)
		require.NoError(t, cmd.Flags().Set("hostname", "ghe.example.com"))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "ghe.example.com", got)
	})

	t.Run("--repo takes priority over --hostname", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", true, true)
		require.NoError(t, cmd.Flags().Set("repo", "cli/cli"))
		require.NoError(t, cmd.Flags().Set("hostname", "ghe.example.com"))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "github.com", got)
	})

	t.Run("returns host from positional arg under gh repo", func(t *testing.T) {
		cmd := newCmd(t, "repo", "view", false, false)
		require.NoError(t, cmd.ParseFlags([]string{"ghe.example.com/owner/repo"}))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "ghe.example.com", got)
	})

	t.Run("does not use positional arg outside of gh repo parent", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		require.NoError(t, cmd.ParseFlags([]string{"ghe.example.com/owner/repo"}))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "", got)
	})

	t.Run("returns host from GH_REPO env", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		t.Setenv("GH_REPO", "ghe.example.com/owner/repo")

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "ghe.example.com", got)
	})

	t.Run("flag takes priority over GH_REPO env", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", true, false)
		t.Setenv("GH_REPO", "ghe.example.com/owner/repo")
		require.NoError(t, cmd.Flags().Set("repo", "cli/cli"))

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "github.com", got)
	})

	t.Run("falls back to BaseRepo from factory", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		t.Setenv("GH_REPO", "")
		baseRepo := func() (ghrepo.Interface, error) {
			return ghrepo.NewWithHost("owner", "repo", "ghe.example.com"), nil
		}

		got := telemetry.GuessTargetHost(cmd, baseRepo, nil)
		assert.Equal(t, "ghe.example.com", got)
	})

	t.Run("ignores BaseRepo when it returns an error", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		t.Setenv("GH_REPO", "")
		baseRepo := func() (ghrepo.Interface, error) {
			return nil, errors.New("no base repo")
		}

		// Without config, we should hit the empty fallback rather than anything
		// derived from baseRepo.
		got := telemetry.GuessTargetHost(cmd, baseRepo, nil)
		assert.Equal(t, "", got)
	})

	t.Run("falls back to default host from config", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		t.Setenv("GH_REPO", "")
		cfg := &ghmock.ConfigMock{
			AuthenticationFunc: func() gh.AuthConfig {
				return &stubAuthConfig{defaultHost: "ghe.example.com"}
			},
		}
		config := func() (gh.Config, error) { return cfg, nil }

		got := telemetry.GuessTargetHost(cmd, nil, config)
		assert.Equal(t, "ghe.example.com", got)
	})

	t.Run("returns empty string when no signal is available", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		t.Setenv("GH_REPO", "")

		got := telemetry.GuessTargetHost(cmd, nil, nil)
		assert.Equal(t, "", got)
	})

	t.Run("returns empty string when config returns an error", func(t *testing.T) {
		cmd := newCmd(t, "pr", "view", false, false)
		t.Setenv("GH_REPO", "")
		config := func() (gh.Config, error) { return nil, errors.New("no config") }

		got := telemetry.GuessTargetHost(cmd, nil, config)
		assert.Equal(t, "", got)
	})
}

// stubAuthConfig is a minimal gh.AuthConfig implementation for tests that only
// exercise the default host lookup. The production gh.AuthConfig is a concrete
// struct with unexported fields so we use the interface that Authentication()
// returns instead.
type stubAuthConfig struct {
	gh.AuthConfig
	defaultHost string
}

func (s *stubAuthConfig) DefaultHost() (string, string) {
	return s.defaultHost, "default"
}
