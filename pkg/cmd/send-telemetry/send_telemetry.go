package sendtelemetry

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/internal/featureflags"
	"github.com/cli/cli/v2/internal/featureflags/cafe"
	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmdutil"
	ghauth "github.com/cli/go-gh/v2/pkg/auth"
	"github.com/spf13/cobra"
)

const (
	defaultCentralEndpointURL       = "https://central.github.com/api/usage/github-cli"
	defaultFeatureFlagEndpointURL   = "https://clientapps.github.com"
	telemetryFeatureFlag            = "gh_cli_telemetry"
)

type SendTelemetryOptions struct {
	CentralEndpointURL     string
	FeatureFlagEndpointURL string
	PayloadJSON            string
	HTTPUnixSocket         string
	AuthToken              string
	CacheDir               string
	IsEnterprise           bool
}

func NewCmdSendTelemetry(f *cmdutil.Factory) *cobra.Command {
	return newCmdSendTelemetry(f, nil)
}

func newCmdSendTelemetry(f *cmdutil.Factory, runF func(*SendTelemetryOptions) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "send-telemetry",
		Short:  "Send telemetry event to Central",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return nil //nolint:nilerr // Best effort telemetry.
			}

			payload, err := io.ReadAll(f.IOStreams.In)
			if err != nil {
				return nil //nolint:nilerr // Best effort telemetry.
			}

			authCfg := cfg.Authentication()
			host, _ := authCfg.DefaultHost()
			token, _ := authCfg.ActiveToken(host)

			opts := &SendTelemetryOptions{
				CentralEndpointURL:     cmp.Or(os.Getenv("CENTRAL_ENDPOINT_URL"), defaultCentralEndpointURL),
				FeatureFlagEndpointURL: cmp.Or(os.Getenv("FEATURE_FLAG_ENDPOINT_URL"), defaultFeatureFlagEndpointURL),
				PayloadJSON:            string(payload),
				// This is a best effort approach to allow telemetry to be sent via a Unix domain socket which is sometimes configured.
				// Technically, there could be a different domain socket per host, but in practice this is unlikely, and it would require
				// plumbing through the host from the parent process, which is often non-trivial. In the case this doesn't work, we'll silently fail.
				HTTPUnixSocket: cfg.HTTPUnixSocket("").Value,
				AuthToken:      token,
				CacheDir:       cfg.CacheDir(),
				IsEnterprise:   ghauth.IsEnterprise(host),
			}

			if runF != nil {
				return runF(opts)
			}
			return runSendTelemetry(opts)
		},
	}

	cmdutil.DisableAuthCheck(cmd)

	return cmd
}

func runSendTelemetry(opts *SendTelemetryOptions) error {
	var event telemetry.Event
	if err := json.Unmarshal([]byte(opts.PayloadJSON), &event); err != nil {
		return nil // Best effort telemetry.
	}

	if !isTelemetryFlagEnabled(opts) {
		return nil
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	if opts.HTTPUnixSocket != "" {
		client.Transport = newUnixDomainSocketRoundTripper(opts.HTTPUnixSocket)
	}

	req, err := http.NewRequest(http.MethodPost, opts.CentralEndpointURL, strings.NewReader(opts.PayloadJSON))
	if err != nil {
		return nil // Best effort telemetry.
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil // Best effort telemetry.
	}
	defer resp.Body.Close()

	return nil
}

// isTelemetryFlagEnabled checks whether the gh_cli_telemetry feature flag is enabled.
// For GHES hosts, feature flags default to off (no telemetry).
// For github.com, it fetches from CAFE with disk caching.
// On any error fetching flags, it fails closed (telemetry is NOT sent).
func isTelemetryFlagEnabled(opts *SendTelemetryOptions) bool {
	if opts.IsEnterprise {
		return false
	}

	if opts.AuthToken == "" {
		return false
	}

	httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &bearerTokenTransport{token: opts.AuthToken, base: http.DefaultTransport},
	}

	cafeClient := cafe.NewClient(httpClient, opts.FeatureFlagEndpointURL)
	ffClient := featureflags.NewClient(cafeClient, opts.CacheDir)

	flags, err := ffClient.GetFeatureFlags(context.Background(), []string{telemetryFeatureFlag})
	if err != nil {
		return false // Fail closed — don't send telemetry if we can't check the flag.
	}

	enabled, ok := flags[telemetryFeatureFlag]
	if !ok {
		return false // Flag not in response — fail closed.
	}

	return enabled
}

type bearerTokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.token))
	return t.base.RoundTrip(req)
}

func newUnixDomainSocketRoundTripper(socketPath string) http.RoundTripper {
	dial := func(network, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}

	return &http.Transport{
		Dial:              dial,
		DialTLS:           dial,
		DisableKeepAlives: true,
	}
}
