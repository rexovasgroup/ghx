//go:build !notelemetry

package config

// defaultTelemetryState controls whether telemetry is enabled or disabled by
// default. In standard builds telemetry is enabled. Distribution packagers
// who wish to ship gh with telemetry disabled by default can build with the
// "notelemetry" build tag instead.
// See internal/config/telemetry_disabled.go and script/build.go.
var defaultTelemetryState = "enabled"
