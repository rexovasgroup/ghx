package sendtelemetry

import (
	"cmp"
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
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}

			opts := &SendTelemetryOptions{
				CentralEndpointURL: cmp.Or(os.Getenv("CENTRAL_ENDPOINT_URL"), defaultCentralEndpointURL),
				PayloadJSON:        args[0],
				// This is a best effort to use a Unix Socket if configured. In most cases, if there is one configured
				// it will be at the global level. However, since Central is not related to a specific host, we can't
				// know that the socket we choose will work.
				HTTPUnixSocket: cfg.HTTPUnixSocket("").Value,
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
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
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
