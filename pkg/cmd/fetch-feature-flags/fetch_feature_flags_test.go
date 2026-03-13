package fetchfeatureflags

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	flags := featureflags.Fetch(cacheDir, "github.com", "testuser", "")
	assert.True(t, flags.Telemetry)
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

	// Then it should return an error
	require.Error(t, err)
}

func TestRunFetchFeatureFlags_fromCache(t *testing.T) {
	// Given a cache with telemetry enabled
	cacheDir := t.TempDir()
	cacheData, _ := json.Marshal(map[string]any{
		"flags":      map[string]bool{"gh_cli_telemetry": true},
		"fetched_at": time.Now().Format(time.RFC3339),
	})
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"), cacheData, 0o600))

	ios, _, stdout, _ := iostreams.Test()
	opts := &FetchFeatureFlagsOptions{
		IO:        ios,
		FromCache: true,
		CacheDir:  cacheDir,
		Host:      "github.com",
		User:      "testuser",
	}

	// When I fetch feature flags with --from-cache
	err := runFetchFeatureFlags(opts)

	// Then it should print the cached flag state
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"Telemetry": true`)
}

func TestNewCmdFetchFeatureFlags(t *testing.T) {
	// Given GH_HOST is set and config has a token for that host
	t.Setenv("GH_HOST", "github.com")

	cfg := config.NewFromString(`
hosts:
  github.com:
    oauth_token: test-token
    user: testuser
`)

	ios, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: ios,
		Config: func() (gh.Config, error) {
			return cfg, nil
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
	assert.Equal(t, "github.com", gotOpts.Host)
	assert.Equal(t, "test-token", gotOpts.AuthToken)
	assert.Equal(t, "testuser", gotOpts.User)
}
