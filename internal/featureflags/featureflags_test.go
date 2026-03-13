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

func writeTestCache(t *testing.T, cacheDir, host, user string, c *cache) {
	t.Helper()
	data, err := json.Marshal(c)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, host+"-"+user+"-feature-flags.json"), data, 0o600))
}

// --- ReadCachedFlags tests ---

func TestReadCachedFlags_freshCache(t *testing.T) {
	// Given a valid cache with telemetry enabled
	cacheDir := t.TempDir()
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   cacheVersion,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: time.Now().Add(-10 * time.Minute),
	})

	// When I read the cached flags
	flags := ReadCachedFlags(cacheDir, "github.com", "testuser")

	// Then telemetry should be enabled
	assert.True(t, flags.Telemetry)
}

func TestReadCachedFlags_noCache(t *testing.T) {
	// Given no cache file exists
	cacheDir := t.TempDir()

	// When I read the cached flags
	flags := ReadCachedFlags(cacheDir, "github.com", "testuser")

	// Then it should return defaults (all disabled)
	assert.False(t, flags.Telemetry)
}

func TestReadCachedFlags_corruptCache(t *testing.T) {
	// Given a corrupt cache file
	cacheDir := t.TempDir()
	path := filepath.Join(cacheDir, "github.com-testuser-feature-flags.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	// When I read the cached flags
	flags := ReadCachedFlags(cacheDir, "github.com", "testuser")

	// Then it should return defaults (all disabled)
	assert.False(t, flags.Telemetry)
}

func TestReadCachedFlags_wrongVersion(t *testing.T) {
	// Given a cache with an incompatible schema version
	cacheDir := t.TempDir()
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   999,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: time.Now(),
	})

	// When I read the cached flags
	flags := ReadCachedFlags(cacheDir, "github.com", "testuser")

	// Then it should return defaults (version mismatch treated as invalid)
	assert.False(t, flags.Telemetry)
}

// --- IsCacheStale tests ---

func TestIsCacheStale_noCache(t *testing.T) {
	// Given no cache file exists
	cacheDir := t.TempDir()

	// Then the cache should be stale
	assert.True(t, IsCacheStale(cacheDir, "github.com", "testuser"))
}

func TestIsCacheStale_freshCache(t *testing.T) {
	// Given a cache fetched 10 minutes ago
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   cacheVersion,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: now.Add(-10 * time.Minute),
	})

	// Then the cache should not be stale
	assert.False(t, isCacheStaleAt(cacheDir, "github.com", "testuser", now))
}

func TestIsCacheStale_expiredCache(t *testing.T) {
	// Given a cache fetched 31 minutes ago
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   cacheVersion,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: now.Add(-31 * time.Minute),
	})

	// Then the cache should be stale
	assert.True(t, isCacheStaleAt(cacheDir, "github.com", "testuser", now))
}

func TestIsCacheStale_wrongVersion(t *testing.T) {
	// Given a cache with wrong schema version
	cacheDir := t.TempDir()
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   999,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: time.Now(),
	})

	// Then the cache should be stale (version mismatch)
	assert.True(t, IsCacheStale(cacheDir, "github.com", "testuser"))
}

// --- FetchAndCache tests ---

func TestFetchAndCache_success(t *testing.T) {
	// Given a CAFE server returning the telemetry flag as enabled
	cacheDir := t.TempDir()
	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	t.Cleanup(server.Close)

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	// When I fetch and cache
	err := client.FetchAndCache(context.Background())

	// Then it should succeed
	require.NoError(t, err)

	// And the cache should be written with the correct version and flags
	c, err := readCache(cachePath(cacheDir, "github.com", "testuser"))
	require.NoError(t, err)
	assert.Equal(t, cacheVersion, c.Version)
	assert.Equal(t, true, c.Flags["gh_cli_telemetry"])
}

func TestFetchAndCache_cafeError(t *testing.T) {
	// Given a CAFE server that returns an error
	cacheDir := t.TempDir()
	handler := cafe.NewViewerAPIServer(&fakeViewerAPI{stubbedErr: assert.AnError})
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	// When I fetch and cache
	err := client.FetchAndCache(context.Background())

	// Then it should return an error
	require.Error(t, err)

	// And no cache file should be written
	_, err = os.Stat(cachePath(cacheDir, "github.com", "testuser"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestFetchAndCache_preservesPriorCacheOnError(t *testing.T) {
	// Given a valid existing cache
	cacheDir := t.TempDir()
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   cacheVersion,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: time.Now().Add(-31 * time.Minute),
	})

	// And a CAFE server that returns an error
	handler := cafe.NewViewerAPIServer(&fakeViewerAPI{stubbedErr: assert.AnError})
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	// When I fetch and cache
	err := client.FetchAndCache(context.Background())

	// Then it should return an error
	require.Error(t, err)

	// And the prior cache should be preserved
	c, err := readCache(cachePath(cacheDir, "github.com", "testuser"))
	require.NoError(t, err)
	assert.Equal(t, true, c.Flags["gh_cli_telemetry"])
}

func TestFetchAndCache_updatesStaleCache(t *testing.T) {
	// Given a stale cache with telemetry disabled
	cacheDir := t.TempDir()
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   cacheVersion,
		Flags:     map[string]bool{"gh_cli_telemetry": false},
		FetchedAt: time.Now().Add(-31 * time.Minute),
	})

	// And a CAFE server returning telemetry enabled
	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	t.Cleanup(server.Close)

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	// When I fetch and cache
	err := client.FetchAndCache(context.Background())

	// Then it should succeed
	require.NoError(t, err)

	// And the cache should be updated with the new value
	c, err := readCache(cachePath(cacheDir, "github.com", "testuser"))
	require.NoError(t, err)
	assert.Equal(t, true, c.Flags["gh_cli_telemetry"])
}

func TestFetchAndCache_noCafeHitWhenNotCalled(t *testing.T) {
	// Given a fresh cache
	cacheDir := t.TempDir()
	writeTestCache(t, cacheDir, "github.com", "testuser", &cache{
		Version:   cacheVersion,
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: time.Now().Add(-10 * time.Minute),
	})

	// When I only read cached flags (no FetchAndCache call)
	flags := ReadCachedFlags(cacheDir, "github.com", "testuser")

	// Then telemetry should be enabled and no network was needed
	assert.True(t, flags.Telemetry)
}
