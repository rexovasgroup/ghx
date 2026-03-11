package sendtelemetry

import (
	"cmp"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

const defaultCentralEndpointURL = "https://central.github.com/api/usage/github-cli"

type SendTelemetryOptions struct {
	CentralEndpointURL string
	PayloadJSON        string
	HTTPUnixSocket     string
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
			config, err := f.Config()
			if err != nil {
				return nil //nolint:nilerr // Best effort telemetry.
			}

			payload, err := io.ReadAll(f.IOStreams.In)
			if err != nil {
				return nil //nolint:nilerr // Best effort telemetry.
			}

			opts := &SendTelemetryOptions{
				CentralEndpointURL: cmp.Or(os.Getenv("CENTRAL_ENDPOINT_URL"), defaultCentralEndpointURL),
				PayloadJSON:        string(payload),
				// This is a best effort approach to allow telemetry to be sent via a Unix domain socket which is sometimes configured.
				// Technically, there could be a different domain socket per host, but in practice this is unlikely, and it would require
				// plumbing through the host from the parent process, which is often non-trivial. In the case this doesn't work, we'll silently fail.
				HTTPUnixSocket: config.HTTPUnixSocket("").Value,
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
		return nil //nolint:nilerr // Best effort telemetry.
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	if opts.HTTPUnixSocket != "" {
		client.Transport = newUnixDomainSocketRoundTripper(opts.HTTPUnixSocket)
	}

	req, err := http.NewRequest(http.MethodPost, opts.CentralEndpointURL, strings.NewReader(opts.PayloadJSON))
	if err != nil {
		return nil //nolint:nilerr // Best effort telemetry.
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil //nolint:nilerr // Best effort telemetry.
	}
	defer resp.Body.Close()

	return nil
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
