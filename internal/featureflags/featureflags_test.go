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

func newTestServer(t *testing.T, flags map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type featureFlag struct {
			Name      string `json:"name"`
			IsEnabled bool   `json:"is_enabled"`
		}
		type resp struct {
			FeatureFlags []featureFlag `json:"feature_flags"`
		}
		var response resp
		for name, enabled := range flags {
			response.FeatureFlags = append(response.FeatureFlags, featureFlag{Name: name, IsEnabled: enabled})
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
}

func TestGetFeatureFlags_freshCache(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": true},
		FetchedAt: now.Add(-10 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, cacheFileName), data, 0o600))

	// No server needed — cache should be used
	cafeClient := cafe.NewClient(http.DefaultClient, "http://localhost:1")
	client := NewClient(cafeClient, stateDir)
	client.now = func() time.Time { return now }

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)
}

func TestGetFeatureFlags_staleCache(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"gh_cli_telemetry": false},
		FetchedAt: now.Add(-31 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, cacheFileName), data, 0o600))

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), server.URL)
	client := NewClient(cafeClient, stateDir)
	client.now = func() time.Time { return now }

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)
}

func TestGetFeatureFlags_noCache(t *testing.T) {
	stateDir := t.TempDir()

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), server.URL)
	client := NewClient(cafeClient, stateDir)

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)

	// Verify cache was written
	data, err := os.ReadFile(filepath.Join(stateDir, cacheFileName))
	require.NoError(t, err)
	var written cache
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, written.Flags)
}

func TestGetFeatureFlags_cacheMissingRequestedFlag(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cached := &cache{
		Flags:     map[string]bool{"other_flag": true},
		FetchedAt: now.Add(-5 * time.Minute),
	}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, cacheFileName), data, 0o600))

	server := newTestServer(t, map[string]bool{"gh_cli_telemetry": true})
	defer server.Close()

	cafeClient := cafe.NewClient(server.Client(), server.URL)
	client := NewClient(cafeClient, stateDir)
	client.now = func() time.Time { return now }

	flags, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"gh_cli_telemetry": true}, flags)
}

func TestGetFeatureFlags_cafeError(t *testing.T) {
	stateDir := t.TempDir()

	cafeClient := cafe.NewClient(http.DefaultClient, "http://localhost:1")
	client := NewClient(cafeClient, stateDir)

	_, err := client.GetFeatureFlags(context.Background(), []string{"gh_cli_telemetry"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching feature flags")
}
