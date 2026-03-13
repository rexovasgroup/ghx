package fetchfeatureflags

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cli/cli/v2/internal/featureflags"
	"github.com/cli/cli/v2/internal/featureflags/cafe"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

const defaultFeatureFlagEndpointURL = "https://clientapps.github.com"

type FetchFeatureFlagsOptions struct {
	FeatureFlagEndpointURL string
	AuthToken              string
	CacheDir               string
	Host                   string
	User                   string
	HTTPUnixSocket         string
}

func NewCmdFetchFeatureFlags(f *cmdutil.Factory) *cobra.Command {
	return newCmdFetchFeatureFlags(f, nil)
}

func newCmdFetchFeatureFlags(f *cmdutil.Factory, runF func(*FetchFeatureFlagsOptions) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "fetch-feature-flags",
		Short:  "Fetch feature flags from CAFE and update the local cache",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return nil //nolint:nilerr // Best effort.
			}

			// The parent process sets GH_HOST to the targeted host.
			authCfg := cfg.Authentication()
			host, _ := authCfg.DefaultHost()
			token, _ := authCfg.ActiveToken(host)
			user, _ := authCfg.ActiveUser(host)

			opts := &FetchFeatureFlagsOptions{
				FeatureFlagEndpointURL: cmp.Or(os.Getenv("FEATURE_FLAG_ENDPOINT_URL"), defaultFeatureFlagEndpointURL),
				AuthToken:              token,
				CacheDir:               cfg.CacheDir(),
				Host:                   host,
				User:                   user,
				HTTPUnixSocket:         cfg.HTTPUnixSocket("").Value,
			}

			if runF != nil {
				return runF(opts)
			}
			return runFetchFeatureFlags(opts)
		},
	}

	cmdutil.DisableAuthCheck(cmd)

	return cmd
}

func runFetchFeatureFlags(opts *FetchFeatureFlagsOptions) error {
	if opts.AuthToken == "" {
		return nil // No token — can't call CAFE.
	}

	httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &bearerTokenTransport{token: opts.AuthToken, base: handleUnixDomainSocket(opts.HTTPUnixSocket)},
	}

	var cafeOpts []cafe.Option
	if opts.FeatureFlagEndpointURL != "" {
		cafeOpts = append(cafeOpts, cafe.WithBaseURL(opts.FeatureFlagEndpointURL))
	}
	cafeClient := cafe.NewClient(httpClient, cafeOpts...)
	ffClient := featureflags.NewClient(cafeClient, opts.CacheDir, opts.Host, opts.User)

	// Best effort — all errors silently ignored.
	_ = ffClient.FetchAndCache(context.Background())
	return nil
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
