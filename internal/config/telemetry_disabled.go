//go:build notelemetry

package config

// defaultTelemetryState controls whether telemetry is enabled or disabled by
// default. The "notelemetry" build tag makes telemetry disabled by default,
// but users can still opt in via:
//
//	gh config set telemetry enabled
//
// or by setting the GH_TELEMETRY environment variable.
// See internal/config/telemetry_enabled.go and script/build.go.
var defaultTelemetryState = "disabled"
