// Package featureflags provides a cached feature flag client backed by the CAFE service.
//
// The intended lifecycle is:
//  1. At startup, load defaults and overlay with cached flags (ReadCachedFlags).
//  2. If the cache is stale (IsCacheStale), spawn an async refresh subprocess.
//  3. The refresh subprocess fetches from CAFE and atomically writes the cache (Client.FetchAndCache).
//  4. The current invocation uses the snapshot from step 1 — flags never change mid-command.
//  5. The next invocation picks up the refreshed cache.
package featureflags

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cli/cli/v2/internal/featureflags/cafe"
)

const (
	defaultCacheTTL = 30 * time.Minute
	cacheVersion    = 1

	flagTelemetry = "gh_cli_telemetry"
)

// allFlagNames is the list of all flag names we request from CAFE.
var allFlagNames = []string{flagTelemetry}

// FlagState holds the resolved state of all feature flags.
// Zero value has all flags disabled, which is the safe default.
type FlagState struct {
	Telemetry bool
}

// cache represents the on-disk feature flag cache.
type cache struct {
	Version   int             `json:"version"`
	Flags     map[string]bool `json:"flags"`
	FetchedAt time.Time       `json:"fetched_at"`
}

func cachePath(cacheDir, host, user string) string {
	return filepath.Join(cacheDir, host+"-"+user+"-feature-flags.json")
}

// readCache reads and validates the cache file. Returns an error if the file
// is missing, corrupt, or has an incompatible schema version.
func readCache(path string) (cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cache{}, err
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return cache{}, err
	}
	if c.Version != cacheVersion {
		return cache{}, fmt.Errorf("cache version mismatch: got %d, want %d", c.Version, cacheVersion)
	}
	return c, nil
}

// writeCache atomically writes the cache to disk using a temp file + rename.
func writeCache(path string, c *cache) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".feature-flags-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func fromMap(flags map[string]bool) FlagState {
	return FlagState{
		Telemetry: flags[flagTelemetry],
	}
}

// ReadCachedFlags loads feature flags from the disk cache.
// Returns defaults (all flags disabled) on any error: missing file, corrupt data,
// schema version mismatch, etc.
func ReadCachedFlags(cacheDir, host, user string) FlagState {
	c, err := readCache(cachePath(cacheDir, host, user))
	if err != nil {
		return FlagState{}
	}
	return fromMap(c.Flags)
}

// IsCacheStale reports whether the cache needs refreshing.
// Returns true if the cache is missing, corrupt, wrong version, or older than the TTL.
func IsCacheStale(cacheDir, host, user string) bool {
	return isCacheStaleAt(cacheDir, host, user, time.Now())
}

func isCacheStaleAt(cacheDir, host, user string, now time.Time) bool {
	c, err := readCache(cachePath(cacheDir, host, user))
	if err != nil {
		return true
	}
	return now.Sub(c.FetchedAt) >= defaultCacheTTL
}

// Client fetches feature flags from the CAFE service and writes them to the disk cache.
// Used by the fetch-feature-flags subprocess.
type Client struct {
	cafe     *cafe.Client
	cacheDir string
	host     string
	user     string
	now      func() time.Time
}

// NewClient creates a feature flag client for fetching and caching flags.
func NewClient(cafeClient *cafe.Client, cacheDir, host, user string) *Client {
	return &Client{
		cafe:     cafeClient,
		cacheDir: cacheDir,
		host:     host,
		user:     user,
		now:      time.Now,
	}
}

// FetchAndCache fetches all feature flags from CAFE, validates the response,
// and atomically writes the cache. If the CAFE response is invalid, the prior
// cache is preserved.
func (c *Client) FetchAndCache(ctx context.Context) error {
	flags, err := c.cafe.GetFeatureFlags(ctx, allFlagNames)
	if err != nil {
		return fmt.Errorf("fetching feature flags from CAFE: %w", err)
	}

	// Validate: ensure we got a non-nil map with expected keys before overwriting cache.
	if flags == nil {
		return fmt.Errorf("CAFE returned nil flags")
	}

	return writeCache(cachePath(c.cacheDir, c.host, c.user), &cache{
		Version:   cacheVersion,
		Flags:     flags,
		FetchedAt: c.now(),
	})
}
