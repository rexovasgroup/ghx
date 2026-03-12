// Package featureflags provides a cached feature flag client backed by the CAFE service.
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

	flagTelemetry = "gh_cli_telemetry"
)

// allFlagNames is the list of all flag names we request from CAFE.
var allFlagNames = []string{flagTelemetry}

// FeatureFlags holds the resolved state of all feature flags.
type FeatureFlags struct {
	Telemetry bool
}

// fromMap populates FeatureFlags from a raw flag map.
func fromMap(flags map[string]bool) FeatureFlags {
	return FeatureFlags{
		Telemetry: flags[flagTelemetry],
	}
}

// cache represents the on-disk feature flag cache.
type cache struct {
	Flags     map[string]bool `json:"flags"`
	FetchedAt time.Time       `json:"fetched_at"`
}

// Client fetches and caches feature flags from the CAFE service.
type Client struct {
	cafe     *cafe.Client
	cacheDir string
	cacheKey string
	cacheTTL time.Duration
	now      func() time.Time
}

// NewClient creates a feature flag client.
// The host and user parameters scope the disk cache so that different accounts
// don't share cached flag values.
func NewClient(cafeClient *cafe.Client, cacheDir, host, user string) *Client {
	return &Client{
		cafe:     cafeClient,
		cacheDir: cacheDir,
		cacheKey: host + "-" + user,
		cacheTTL: defaultCacheTTL,
		now:      time.Now,
	}
}

// Get fetches all feature flags, using the disk cache when fresh.
// On any error fetching from CAFE, it returns an error — callers decide fail-open/closed behavior.
func (c *Client) Get(ctx context.Context) (FeatureFlags, error) {
	cached, err := c.readCache()
	if err == nil && c.isCacheFresh(cached) && c.cacheHasAllFlags(cached, allFlagNames) {
		return fromMap(cached.Flags), nil
	}

	flags, err := c.cafe.GetFeatureFlags(ctx, allFlagNames)
	if err != nil {
		return FeatureFlags{}, fmt.Errorf("fetching feature flags: %w", err)
	}

	_ = c.writeCache(&cache{
		Flags:     flags,
		FetchedAt: c.now(),
	})

	return fromMap(flags), nil
}

func (c *Client) isCacheFresh(cached *cache) bool {
	return c.now().Sub(cached.FetchedAt) < c.cacheTTL
}

func (c *Client) cacheHasAllFlags(cached *cache, flagNames []string) bool {
	for _, name := range flagNames {
		if _, ok := cached.Flags[name]; !ok {
			return false
		}
	}
	return true
}

func (c *Client) cachePath() string {
	return filepath.Join(c.cacheDir, c.cacheKey+"-feature-flags.json")
}

func (c *Client) readCache() (*cache, error) {
	data, err := os.ReadFile(c.cachePath())
	if err != nil {
		return nil, err
	}
	var cached cache
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}
	return &cached, nil
}

func (c *Client) writeCache(cached *cache) error {
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.cachePath(), data, 0o600)
}
