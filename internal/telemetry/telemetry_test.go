package telemetry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func stubDeviceID(id string) func() {
	orig := deviceIDFunc
	deviceIDFunc = func() (string, error) { return id, nil }
	return func() { deviceIDFunc = orig }
}

func stubDeviceIDError() func() {
	orig := deviceIDFunc
	deviceIDFunc = func() (string, error) { return "", errors.New("no machine id") }
	return func() { deviceIDFunc = orig }
}

func stubStateDir(dir string) func() {
	orig := stateDirFunc
	stateDirFunc = func() string { return dir }
	return func() { stateDirFunc = orig }
}

func TestBuildEventPayloadNilCommand(t *testing.T) {
	// BuildEventPayload no longer accepts nil cmd — that case is handled by the caller.
}

func TestBuildEventPayloadPopulatesDimensions(t *testing.T) {
	t.Cleanup(stubDeviceID("test-device-id"))

	root := &cobra.Command{Use: "gh"}
	pr := &cobra.Command{Use: "pr"}
	create := &cobra.Command{Use: "create"}
	root.AddCommand(pr)
	pr.AddCommand(create)

	event, err := BuildEventPayload(create, "2.45.0")
	require.NoError(t, err)

	want := Event{
		EventType: "usage",
		Dimensions: Dimensions{
			Command:      "gh pr create",
			DeviceID:     "test-device-id",
			Flags:        "",
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
			Version:      "2.45.0",
		},
	}
	require.Equal(t, want, event)
}

func TestBuildEventPayloadStripsVersionPrefix(t *testing.T) {
	t.Cleanup(stubDeviceID("test-device-id"))

	cmd := &cobra.Command{Use: "gh"}
	event, err := BuildEventPayload(cmd, "v2.45.0")
	require.NoError(t, err)
	require.NotNil(t, event)
	require.Equal(t, "2.45.0", event.Dimensions.Version)
}

func TestBuildEventPayloadReturnsNilWhenDeviceIDFails(t *testing.T) {
	t.Cleanup(stubDeviceIDError())

	cmd := &cobra.Command{Use: "gh"}
	_, err := BuildEventPayload(cmd, "2.45.0")
	require.Error(t, err)
}

func TestBuildEventPayloadCommandPath(t *testing.T) {
	t.Cleanup(stubDeviceID("test-device-id"))

	tests := []struct {
		name  string
		setup func() *cobra.Command
		want  string
	}{
		{
			name: "subcommand",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "gh"}
				issue := &cobra.Command{Use: "issue"}
				list := &cobra.Command{Use: "list"}
				root.AddCommand(issue)
				issue.AddCommand(list)
				return list
			},
			want: "gh issue list",
		},
		{
			name: "top-level command",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "gh"}
				status := &cobra.Command{Use: "status"}
				root.AddCommand(status)
				return status
			},
			want: "gh status",
		},
		{
			name: "root command itself",
			setup: func() *cobra.Command {
				return &cobra.Command{Use: "gh"}
			},
			want: "gh",
		},
		{
			name: "flags are not included in command path",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "gh"}
				pr := &cobra.Command{Use: "pr"}
				list := &cobra.Command{Use: "list"}
				list.Flags().StringP("state", "s", "open", "Filter by state")
				root.AddCommand(pr)
				pr.AddCommand(list)
				list.ParseFlags([]string{"--state", "closed"})
				return list
			},
			want: "gh pr list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setup()
			event, err := BuildEventPayload(cmd, "1.0.0")
			require.NoError(t, err)
			require.NotNil(t, event)
			require.Equal(t, tt.want, event.Dimensions.Command)
		})
	}
}

func TestBuildEventPayloadCollectsFlags(t *testing.T) {
	t.Cleanup(stubDeviceID("test-device-id"))

	tests := []struct {
		name      string
		setup     func() *cobra.Command
		wantFlags string
	}{
		{
			name: "no flags set",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "list"}
				cmd.Flags().StringP("state", "s", "open", "Filter by state")
				cmd.Flags().IntP("limit", "L", 30, "Maximum number of items")
				return cmd
			},
			wantFlags: "",
		},
		{
			name: "single flag set",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "list"}
				cmd.Flags().StringP("state", "s", "open", "Filter by state")
				cmd.Flags().IntP("limit", "L", 30, "Maximum number of items")
				cmd.ParseFlags([]string{"--state", "closed"})
				return cmd
			},
			wantFlags: "state",
		},
		{
			name: "multiple flags set are sorted",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "list"}
				cmd.Flags().StringP("state", "s", "open", "Filter by state")
				cmd.Flags().IntP("limit", "L", 30, "Maximum number of items")
				cmd.Flags().BoolP("web", "w", false, "Open in browser")
				cmd.ParseFlags([]string{"--web", "--limit", "10", "--state", "closed"})
				return cmd
			},
			wantFlags: "limit,state,web",
		},
		{
			name: "only explicitly set flags are included",
			setup: func() *cobra.Command {
				cmd := &cobra.Command{Use: "list"}
				cmd.Flags().StringP("state", "s", "open", "Filter by state")
				cmd.Flags().IntP("limit", "L", 30, "Maximum number of items")
				cmd.Flags().BoolP("web", "w", false, "Open in browser")
				cmd.ParseFlags([]string{"--limit", "10"})
				return cmd
			},
			wantFlags: "limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setup()
			event, err := BuildEventPayload(cmd, "1.0.0")
			require.NoError(t, err)
			require.NotNil(t, event)
			require.Equal(t, tt.wantFlags, event.Dimensions.Flags)
		})
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	event := Event{
		EventType: "usage",
		Dimensions: Dimensions{
			Command:      "gh repo clone",
			DeviceID:     "abc123hashed",
			Flags:        "depth,upstream-remote-name",
			OS:           "linux",
			Architecture: "amd64",
			Version:      "2.44.0",
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded Event
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, event, decoded)
}

func TestGetOrCreateDeviceID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Cleanup(stubStateDir(tmpDir))

	t.Run("creates new ID on first call", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Cleanup(stubStateDir(tmpDir))

		id, err := getOrCreateDeviceID()
		require.NoError(t, err)
		require.NotEmpty(t, id)

		data, err := os.ReadFile(filepath.Join(tmpDir, deviceIDFileName))
		require.NoError(t, err)
		require.Equal(t, id, string(data))
	})

	t.Run("returns same ID on subsequent calls", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Cleanup(stubStateDir(tmpDir))

		id1, err := getOrCreateDeviceID()
		require.NoError(t, err)

		id2, err := getOrCreateDeviceID()
		require.NoError(t, err)

		require.Equal(t, id1, id2, "IDs differ")
	})

	t.Run("trims whitespace from stored ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Cleanup(stubStateDir(tmpDir))

		expected := "some-device-id"
		err := os.WriteFile(filepath.Join(tmpDir, deviceIDFileName), []byte("  some-device-id\n"), 0o600)
		require.NoError(t, err)

		id, err := getOrCreateDeviceID()
		require.NoError(t, err)
		require.Equal(t, expected, id)
	})

	t.Run("returns error for non-ErrNotExist read failures", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Cleanup(stubStateDir(tmpDir))

		// Create device-id as a directory so ReadFile fails with a non-ErrNotExist error.
		err := os.Mkdir(filepath.Join(tmpDir, deviceIDFileName), 0o755)
		require.NoError(t, err)

		_, err = getOrCreateDeviceID()
		require.Error(t, err)
		require.False(t, errors.Is(err, os.ErrNotExist))
	})
}

func TestIsTelemetryEnabledNilAnnotations(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	// Annotations is nil by default on a new command.
	require.False(t, IsTelemetryEnabled(cmd))
}

func stubLookupEnv(fn func(string) (string, bool)) func() {
	orig := lookupEnvFunc
	lookupEnvFunc = fn
	return func() { lookupEnvFunc = orig }
}

func TestIsOptedOut(t *testing.T) {
	envSet := func(val string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			require.Equal(t, "GH_NO_TELEMETRY", key)
			return val, true
		}
	}
	envUnset := func(key string) (string, bool) {
		return "", false
	}

	tests := []struct {
		name              string
		lookupEnv         func(string) (string, bool)
		configNoTelemetry string
		want              bool
	}{
		{
			name:              "env var unset, config empty",
			lookupEnv:         envUnset,
			configNoTelemetry: "",
			want:              false,
		},
		{
			name:              "env var set to true",
			lookupEnv:         envSet("true"),
			configNoTelemetry: "",
			want:              true,
		},
		{
			name:              "env var set to 1",
			lookupEnv:         envSet("1"),
			configNoTelemetry: "",
			want:              true,
		},
		{
			name:              "env var set to yes",
			lookupEnv:         envSet("yes"),
			configNoTelemetry: "",
			want:              true,
		},
		{
			name:              "env var set to any truthy value",
			lookupEnv:         envSet("anything"),
			configNoTelemetry: "",
			want:              true,
		},
		{
			name:              "env var set to false",
			lookupEnv:         envSet("false"),
			configNoTelemetry: "",
			want:              false,
		},
		{
			name:              "env var set to 0",
			lookupEnv:         envSet("0"),
			configNoTelemetry: "",
			want:              false,
		},
		{
			name:              "env var set to no",
			lookupEnv:         envSet("no"),
			configNoTelemetry: "",
			want:              false,
		},
		{
			name:              "env var set to empty string",
			lookupEnv:         envSet(""),
			configNoTelemetry: "",
			want:              false,
		},
		{
			name:              "config set to true",
			lookupEnv:         envUnset,
			configNoTelemetry: "true",
			want:              true,
		},
		{
			name:              "config set to 1",
			lookupEnv:         envUnset,
			configNoTelemetry: "1",
			want:              true,
		},
		{
			name:              "config set to yes",
			lookupEnv:         envUnset,
			configNoTelemetry: "yes",
			want:              true,
		},
		{
			name:              "config set to false",
			lookupEnv:         envUnset,
			configNoTelemetry: "false",
			want:              false,
		},
		{
			name:              "config set to 0",
			lookupEnv:         envUnset,
			configNoTelemetry: "0",
			want:              false,
		},
		{
			name:              "env var takes precedence over config (env truthy, config not set)",
			lookupEnv:         envSet("1"),
			configNoTelemetry: "",
			want:              true,
		},
		{
			name:              "env var takes precedence over config (env falsy, config true)",
			lookupEnv:         envSet("false"),
			configNoTelemetry: "true",
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(stubLookupEnv(tt.lookupEnv))
			got := IsOptedOut(tt.configNoTelemetry)
			require.Equal(t, tt.want, got)
		})
	}
}
