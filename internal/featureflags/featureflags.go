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
	cacheFileName    = "feature-flags.json"
	defaultCacheTTL  = 30 * time.Minute
)

// cache represents the on-disk feature flag cache.
type cache struct {
	Flags     map[string]bool `json:"flags"`
	FetchedAt time.Time       `json:"fetched_at"`
}

// Client fetches and caches feature flags from the CAFE service.
type Client struct {
	cafe     *cafe.Client
	cacheDir string
	cacheTTL time.Duration
	now      func() time.Time
}

// NewClient creates a feature flag client.
func NewClient(cafeClient *cafe.Client, cacheDir string) *Client {
	return &Client{
		cafe:     cafeClient,
		cacheDir: cacheDir,
		cacheTTL: defaultCacheTTL,
		now:      time.Now,
	}
}

// GetFeatureFlags returns the enabled state of the requested flags.
// It reads from disk cache first and only calls CAFE if the cache is stale or missing.
// On any error fetching from CAFE, it returns an error — callers decide fail-open/closed behavior.
func (c *Client) GetFeatureFlags(ctx context.Context, flagNames []string) (map[string]bool, error) {
	cached, err := c.readCache()
	if err == nil && c.isCacheFresh(cached) && c.cacheHasAllFlags(cached, flagNames) {
		result := make(map[string]bool, len(flagNames))
		for _, name := range flagNames {
			result[name] = cached.Flags[name]
		}
		return result, nil
	}

	flags, err := c.cafe.GetFeatureFlags(ctx, flagNames)
	if err != nil {
		return nil, fmt.Errorf("fetching feature flags: %w", err)
	}

	_ = c.writeCache(&cache{
		Flags:     flags,
		FetchedAt: c.now(),
	})

	return flags, nil
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
	return filepath.Join(c.cacheDir, cacheFileName)
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
