package fetchfeatureflags

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/internal/featureflags"
	"github.com/cli/cli/v2/internal/featureflags/cafe"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeViewerAPI struct {
	flags      []*cafe.FeatureFlag
	stubbedErr error
}

func (f *fakeViewerAPI) GetDetails(context.Context, *cafe.GetDetailsRequest) (*cafe.GetDetailsResponse, error) {
	return nil, nil
}

func (f *fakeViewerAPI) GetFeatureFlags(_ context.Context, _ *cafe.GetFeatureFlagsRequest) (*cafe.GetFeatureFlagsResponse, error) {
	if f.stubbedErr != nil {
		return nil, f.stubbedErr
	}
	return &cafe.GetFeatureFlagsResponse{FeatureFlags: f.flags}, nil
}

func newCAFEServer(t *testing.T, flags map[string]bool) *httptest.Server {
	t.Helper()
	var cafeFlags []*cafe.FeatureFlag
	for name, enabled := range flags {
		cafeFlags = append(cafeFlags, &cafe.FeatureFlag{Name: name, IsEnabled: enabled})
	}
	handler := cafe.NewViewerAPIServer(&fakeViewerAPI{flags: cafeFlags})
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	return httptest.NewServer(mux)
}

func TestRunFetchFeatureFlags_success(t *testing.T) {
	// Given a CAFE server returning telemetry enabled
	cacheDir := t.TempDir()
	server := newCAFEServer(t, map[string]bool{"gh_cli_telemetry": true})
	t.Cleanup(server.Close)

	opts := &FetchFeatureFlagsOptions{
		FeatureFlagEndpointURL: server.URL,
		AuthToken:              "test-token",
		CacheDir:               cacheDir,
		Host:                   "github.com",
		User:                   "testuser",
	}

	// When I fetch feature flags
	err := runFetchFeatureFlags(opts)

	// Then it should succeed and cache should be populated
	require.NoError(t, err)
	flags := featureflags.ReadCachedFlags(cacheDir, "github.com", "testuser")
	assert.True(t, flags.Telemetry)
}

func TestRunFetchFeatureFlags_noToken(t *testing.T) {
	// Given no auth token
	opts := &FetchFeatureFlagsOptions{
		AuthToken: "",
		CacheDir:  t.TempDir(),
		Host:      "github.com",
		User:      "testuser",
	}

	// When I fetch feature flags
	err := runFetchFeatureFlags(opts)

	// Then it should return nil (silent skip)
	require.NoError(t, err)

	// And no cache should be written
	flags := featureflags.ReadCachedFlags(opts.CacheDir, "github.com", "testuser")
	assert.False(t, flags.Telemetry)
}

func TestRunFetchFeatureFlags_cafeError(t *testing.T) {
	// Given a CAFE server that errors
	cacheDir := t.TempDir()
	handler := cafe.NewViewerAPIServer(&fakeViewerAPI{stubbedErr: assert.AnError})
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	opts := &FetchFeatureFlagsOptions{
		FeatureFlagEndpointURL: server.URL,
		AuthToken:              "test-token",
		CacheDir:               cacheDir,
		Host:                   "github.com",
		User:                   "testuser",
	}

	// When I fetch feature flags
	err := runFetchFeatureFlags(opts)

	// Then it should still return nil (errors silently ignored)
	require.NoError(t, err)
}

func TestNewCmdFetchFeatureFlags(t *testing.T) {
	// Given a factory
	ios, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: ios,
		Config: func() (gh.Config, error) {
			return config.NewBlankConfig(), nil
		},
	}

	var gotOpts *FetchFeatureFlagsOptions
	cmd := newCmdFetchFeatureFlags(f, func(opts *FetchFeatureFlagsOptions) error {
		gotOpts = opts
		return nil
	})
	cmd.SetArgs([]string{})
	cmd.SetIn(&bytes.Buffer{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// When I execute the command
	_, err := cmd.ExecuteC()

	// Then it should succeed and populate options
	require.NoError(t, err)
	require.NotNil(t, gotOpts)
	assert.Equal(t, defaultFeatureFlagEndpointURL, gotOpts.FeatureFlagEndpointURL)
}
