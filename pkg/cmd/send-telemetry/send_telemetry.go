package sendtelemetry

import (
	"cmp"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

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
			cfg, err := f.Config()
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
				HTTPUnixSocket:     cfg.HTTPUnixSocket("").Value,
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
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: handleUnixDomainSocket(opts.HTTPUnixSocket),
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

func handleUnixDomainSocket(socketPath string) http.RoundTripper {
	if socketPath == "" {
		return http.DefaultTransport
	}

	dial := func(network, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}

	return &http.Transport{
		Dial:              dial,
		DialTLS:           dial,
		DisableKeepAlives: true,
	}
}
