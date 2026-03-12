// Package cafe provides a client for the CAFE (Client Apps Front End) feature flag service.
// CAFE exposes a Twirp API that supports JSON transport, so we use plain HTTP+JSON
// rather than importing the full protobuf/twirp stack.
package cafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	defaultBaseURL     = "https://clientapps.github.com"
	getFeatureFlagsURL = "/twirp/clientappsfe.identity.v1.ViewerAPI/GetFeatureFlags"
)

type getFeatureFlagsRequest struct {
	FlagNames []string `json:"flag_names"`
}

type getFeatureFlagsResponse struct {
	FeatureFlags []featureFlag `json:"feature_flags"`
}

type featureFlag struct {
	Name      string `json:"name"`
	IsEnabled bool   `json:"is_enabled"`
}

// Client talks to the CAFE service to fetch feature flags.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a CAFE client. If baseURL is empty, the default is used.
// The httpClient must already have authentication configured (e.g. Bearer token transport).
func NewClient(httpClient *http.Client, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// GetFeatureFlags fetches the given flag names from CAFE and returns a map of flag name to enabled state.
func (c *Client) GetFeatureFlags(ctx context.Context, flagNames []string) (map[string]bool, error) {
	reqBody := getFeatureFlagsRequest{FlagNames: flagNames}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	url := c.baseURL + getFeatureFlagsURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from CAFE", resp.StatusCode)
	}

	var respBody getFeatureFlagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	flags := make(map[string]bool, len(respBody.FeatureFlags))
	for _, f := range respBody.FeatureFlags {
		flags[f.Name] = f.IsEnabled
	}
	return flags, nil
}
