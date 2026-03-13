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

	ff, err := client.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)
	assert.False(t, cafeHit, "CAFE should not be called when cache is fresh")
}

func TestGet_staleCache(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": false},
		FetchedAt: now.Add(-31 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"), data, 0o600))

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")
	client.now = func() time.Time { return now }

	ff, err := client.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)
}

func TestGet_noCache(t *testing.T) {
	cacheDir := t.TempDir()

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	ff, err := client.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)

	// Verify cache was written
	data, err := os.ReadFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"))
	require.NoError(t, err)
	var written cache
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, true, written.Flags["gh_cli_telemetry"])
}

func TestGet_cacheMissingRequestedFlag(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"other_flag": true},
		FetchedAt: now.Add(-5 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "github.com-testuser-feature-flags.json"), data, 0o600))

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")
	client.now = func() time.Time { return now }

	ff, err := client.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ff.Telemetry)
}

func TestGet_cafeError(t *testing.T) {
	cacheDir := t.TempDir()

	cafeClient := cafe.NewClient(http.DefaultClient, cafe.WithBaseURL("http://localhost:1"))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	_, err := client.Get(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching feature flags")
}

func TestGet_telemetryDisabled(t *testing.T) {
	cacheDir := t.TempDir()

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": false})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), cafe.WithBaseURL(server.URL))
	client := NewClient(cafeClient, cacheDir, "github.com", "testuser")

	ff, err := client.Get(context.Background())
	require.NoError(t, err)
	assert.False(t, ff.Telemetry)
}
