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

func newTestServer(t *testing.T, flags map[string]bool) *httptest.Server {
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

func TestGet_freshCache(t *testing.T) {
	// Given a cache with data newer than the TTL
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: now.Add(-10 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"), data, 0o600))

	var cafeHit bool
	spy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cafeHit = true
	}))
	defer spy.Close()

	cafeClient := cafe.NewClient(spy.Client(), cafe.WithBaseURL(spy.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")
	client.now = func() time.Time { return now }

	// When I fetch the feature flags
	ff, err := client.Get(context.Background())

	// Then it should succeed and return the cached value without calling CAFE
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)
	assert.False(t, cafeHit, "CAFE should not be called when cache is fresh")
}

func TestGet_staleCache(t *testing.T) {
	// Given a cache with data older than the TTL
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": false},
		FetchedAt: now.Add(-31 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"), data, 0o600))

	// And a CAFE server returning the flag as enabled
	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")
	client.now = func() time.Time { return now }

	// When I fetch the feature flags
	ff, err := client.Get(context.Background())

	// Then it should return the fresh value from CAFE, not the stale cache
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)

	// And the cache on disk should be updated with the new value
	updatedData, err := os.ReadFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"))
	require.NoError(t, err)
	var updated cache
	require.NoError(t, json.Unmarshal(updatedData, &updated))
	assert.Equal(t, true, updated.Flags["gh_cli_telemetry"])
	assert.Equal(t, now, updated.FetchedAt.UTC())
}

func TestGet_noCache(t *testing.T) {
	// Given no cache exists
	cacheDir := t.TempDir()

	// And a CAFE server returning the flag as enabled
	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	// When I fetch the feature flags
	ff, err := client.Get(context.Background())

	// Then it should return the value from CAFE
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)

	// And the cache should be written to disk
	data, err := os.ReadFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"))
	require.NoError(t, err)
	var written cache
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, true, written.Flags["gh_cli_telemetry"])
}

func TestGet_cacheMissingRequestedFlag(t *testing.T) {
	// Given a fresh cache that does not contain the requested flag
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"other_flag": true},
		FetchedAt: now.Add(-5 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"), data, 0o600))

	// And a CAFE server returning the flag as enabled
	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")
	client.now = func() time.Time { return now }

	// When I fetch the feature flags
	ff, err := client.Get(context.Background())

	// Then it should fetch from CAFE and return the flag value
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)

	// And the cache on disk should be updated with the new value
	updatedData, err := os.ReadFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"))
	require.NoError(t, err)
	var updated cache
	require.NoError(t, json.Unmarshal(updatedData, &updated))
	assert.Equal(t, true, updated.Flags["gh_cli_telemetry"])
	assert.Equal(t, now, updated.FetchedAt.UTC())
}

func TestGet_cafeError(t *testing.T) {
	// Given no cache exists and CAFE returns an error
	cacheDir := t.TempDir()

	handler := cafe.NewViewerAPIServer(&fakeViewerAPI{stubbedErr: assert.AnError})
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	// When I fetch the feature flags
	_, err := client.Get(context.Background())

	// Then it should return an error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching feature flags")

	// And no cache file should be written
	_, err = os.ReadFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
