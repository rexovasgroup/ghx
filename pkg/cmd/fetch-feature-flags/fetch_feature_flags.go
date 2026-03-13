package fetchfeatureflags

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cli/cli/v2/internal/featureflags"
	"github.com/cli/cli/v2/internal/featureflags/cafe"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

const defaultFeatureFlagEndpointURL = "https://clientapps.github.com"

type FetchFeatureFlagsOptions struct {
	IO                     *iostreams.IOStreams
	FeatureFlagEndpointURL string
	AuthToken              string
	CacheDir               string
	Host                   string
	User                   string
	HTTPUnixSocket         string
	FromCache              bool
}

func NewCmdFetchFeatureFlags(f *cmdutil.Factory) *cobra.Command {
	return newCmdFetchFeatureFlags(f, nil)
}

func newCmdFetchFeatureFlags(f *cmdutil.Factory, runF func(*FetchFeatureFlagsOptions) error) *cobra.Command {
	opts := &FetchFeatureFlagsOptions{
		IO: f.IOStreams,
	}

	cmd := &cobra.Command{
		Use:    "fetch-feature-flags",
		Short:  "Fetch feature flags from CAFE and update the local cache",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}

			// The parent process sets GH_HOST to the targeted host.
			host := os.Getenv("GH_HOST")
			if host == "" {
				return errors.New("GH_HOST environment variable must be set")
			}

			authCfg := cfg.Authentication()
			token, _ := authCfg.ActiveToken(host)
			if token == "" {
				return errors.New("expected to have a token")
			}

			user, err := authCfg.ActiveUser(host)
			if err != nil {
				return err
			}

			opts.FeatureFlagEndpointURL = cmp.Or(os.Getenv("FEATURE_FLAG_ENDPOINT_URL"), defaultFeatureFlagEndpointURL)
			opts.AuthToken = token
			opts.CacheDir = cfg.CacheDir()
			opts.Host = host
			opts.User = user
			opts.HTTPUnixSocket = cfg.HTTPUnixSocket(host).Value

			if runF != nil {
				return runF(opts)
			}
			return runFetchFeatureFlags(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.FromCache, "from-cache", false, "Print cached feature flags instead of fetching from remote")

	return cmd
}

func runFetchFeatureFlags(opts *FetchFeatureFlagsOptions) error {
	if opts.FromCache {
		flags := featureflags.Fetch(opts.CacheDir, opts.Host, opts.User, "")
		flagStr, err := json.MarshalIndent(flags, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(opts.IO.Out, "%s\n", flagStr)
		return nil
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

	return ffClient.FetchAndCache(context.Background())
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
