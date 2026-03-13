// Package telemetry provides best-effort usage telemetry for gh commands.
//
// Telemetry is sent by spawning a detached `gh send-telemetry` subprocess from
// a PersistentPostRun hook on the root Cobra command. This has several known
// limitations:
//
//   - Telemetry is only sent on successful command completion. Commands that
//     are interrupted (e.g. Ctrl+C) or fail with an error do not trigger the
//     PersistentPostRun hook, so no event is recorded.
package telemetry

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cli/cli/v2/internal/config"
	ghauth "github.com/cli/go-gh/v2/pkg/auth"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const deviceIDFileName = "device-id"

// Host type constants used in the HostType dimension.
const (
	HostTypeDotcom     = "dotcom"
	HostTypeTenant     = "tenant"
	HostTypeEnterprise = "enterprise"
)

// ClassifyHost returns the host type for a given hostname.
func ClassifyHost(host string) string {
	if ghauth.IsTenancy(host) {
		return HostTypeTenant
	}
	if ghauth.IsEnterprise(host) {
		return HostTypeEnterprise
	}
	return HostTypeDotcom
}

// stateDirFunc returns the state directory path. Can be replaced in tests.
var stateDirFunc = config.StateDir

// deviceIDFunc returns a per-user device identifier stored in the state directory.
// It generates and persists a UUID on first call. Can be replaced in tests.
var deviceIDFunc = getOrCreateDeviceID

func getOrCreateDeviceID() (string, error) {
	stateDir := stateDirFunc()
	idPath := filepath.Join(stateDir, deviceIDFileName)

	data, err := os.ReadFile(idPath)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	id := uuid.New().String()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(idPath, []byte(id), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

// Event represents a Central usage event payload.
// This struct is marshaled to JSON by the caller and unmarshaled by the
// send-telemetry command, providing type safety across the process boundary.
type Event struct {
	EventType  string     `json:"eventType"`
	Dimensions Dimensions `json:"dimensions"`
}

// Dimensions contains the metadata sent alongside a usage event to Central.
type Dimensions struct {
	// Command is the command name including "gh" down to the subcommand, e.g. "gh pr create".
	Command string `json:"command"`
	// DeviceID is the UUID associated with the user/device combination, e.g. "1e9a73a6-c8bd-4e1e-be02-78f4b11de4e1".
	DeviceID string `json:"device_id"`
	// Flags is a comma-separated sorted list of flag names that were explicitly provided, e.g. "draft,limit,state".
	// Only flag names are recorded, never values.
	Flags string `json:"flags"`
	// OS is the operating system name from runtime.GOOS, e.g. "linux", "darwin", or "windows".
	OS string `json:"os"`
	// Architecture is the CPU architecture from runtime.GOARCH, e.g. "amd64" or "arm64".
	Architecture string `json:"architecture"`
	// Version is the gh CLI version with any "v" prefix stripped, e.g. "2.87.3".
	Version string `json:"version"`
}

// BuildEventPayload constructs the event payload for tracking a command invocation.
// Returns nil if device ID cannot be determined.
func BuildEventPayload(cmd *cobra.Command, version string) (Event, error) {
	deviceID, err := deviceIDFunc()
	if err != nil {
		return Event{}, err
	}

	var flags []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags = append(flags, f.Name)
	})
	slices.Sort(flags)

	return Event{
		EventType: "usage",
		Dimensions: Dimensions{
			Command:      cmd.CommandPath(),
			DeviceID:     deviceID,
			Flags:        strings.Join(flags, ","),
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
			Version:      strings.TrimPrefix(version, "v"),
		},
	}, nil
}

// SpawnSendTelemetry spawns a subprocess to send telemetry via stdin.
// All errors are silently ignored since telemetry is best-effort.
func SpawnSendTelemetry(executable, payloadJSON string) {
	cmd := exec.Command(executable, "send-telemetry", payloadJSON)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return
	}
	_ = cmd.Process.Release() //nolint:errcheck // Best effort telemetry.
}

const telemetryAnnotation = "telemetry"

// EnableTelemetry opts a command into telemetry collection.
//
// During the initial rollout, telemetry is opt-in per command. In the future,
// the default should be swapped so that telemetry is enabled for all commands
// unless explicitly disabled.
func EnableTelemetry(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[telemetryAnnotation] = "true"
}

// IsTelemetryEnabled checks whether telemetry is enabled for the given command.
func IsTelemetryEnabled(cmd *cobra.Command) bool {
	return cmd.Annotations[telemetryAnnotation] == "true"
}

// falseyValues are values considered falsy for environment variable checks.
var falseyValues = []string{"", "0", "false", "no"}

// lookupEnvFunc wraps os.LookupEnv. Can be replaced in tests.
var lookupEnvFunc = os.LookupEnv

// IsOptedOut reports whether the user has opted out of telemetry.
//
// The GH_NO_TELEMETRY environment variable takes precedence: if set to a truthy
// value, telemetry is disabled. Otherwise, the no_telemetry config value is
// checked using the same truthiness rules.
func IsOptedOut(configNoTelemetry string) bool {
	if envVal, ok := lookupEnvFunc("GH_NO_TELEMETRY"); ok {
		return !slices.Contains(falseyValues, envVal)
	}

	return !slices.Contains(falseyValues, configNoTelemetry)
}
