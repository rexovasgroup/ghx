package featureflags

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cli/cli/v2/internal/featureflags/cafe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeViewerAPI implements cafe.ViewerAPI for testing.
type fakeViewerAPI struct {
	flags []*cafe.FeatureFlag
}

func (f *fakeViewerAPI) GetDetails(context.Context, *cafe.GetDetailsRequest) (*cafe.GetDetailsResponse, error) {
	return nil, nil
}

func (f *fakeViewerAPI) GetFeatureFlags(_ context.Context, _ *cafe.GetFeatureFlagsRequest) (*cafe.GetFeatureFlagsResponse, error) {
	return &cafe.GetFeatureFlagsResponse{FeatureFlags: f.flags}, nil
}

func newTestServer(t *testing.T, flags map[string]bool) *httptest.Server {
	t.Helper()
	var protoFlags []*cafe.FeatureFlag
	for name, enabled := range flags {
		protoFlags = append(protoFlags, &cafe.FeatureFlag{Name: name, IsEnabled: enabled})
	}
	handler := cafe.NewViewerAPIServer(&fakeViewerAPI{flags: protoFlags})
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	return httptest.NewServer(mux)
}

func TestGetFeatureFlags_freshCache(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: now.Add(-10 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cacheFileName), data, 0o600))

	// No server needed — cache should be used
	cafeClient := cafe.NewClient(http.DefaultClient, "http://localhost:1")
	client := NewClient(cafeClient, cacheDir)
	client.now = func() time.Time { return now }

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)
}

func TestGetFeatureFlags_staleCache(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": false},
		FetchedAt: now.Add(-31 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cacheFileName), data, 0o600))

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), server.URL)
	client := NewClient(cafeClient, cacheDir)
	client.now = func() time.Time { return now }

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)
}

func TestGetFeatureFlags_noCache(t *testing.T) {
	cacheDir := t.TempDir()

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), server.URL)
	client := NewClient(cafeClient, cacheDir)

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)

	// Verify cache was written
	data, err := os.ReadFile(filepath.Join(cacheDir, cacheFileName))
	require.NoError(t, err)
	var written cache
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, written.Flags)
}

func TestGetFeatureFlags_cacheMissingRequestedFlag(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"other_flag": true},
		FetchedAt: now.Add(-5 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cacheFileName), data, 0o600))

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), server.URL)
	client := NewClient(cafeClient, cacheDir)
	client.now = func() time.Time { return now }

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)
}

func TestGetFeatureFlags_cafeError(t *testing.T) {
	cacheDir := t.TempDir()

	cafeClient := cafe.NewClient(http.DefaultClient, "http://localhost:1")
	client := NewClient(cafeClient, cacheDir)

	_, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching feature flags")
}
