package cafe

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFeatureFlags(t *testing.T) {
	tests := []struct {
		name           string
		flagNames      []string
		serverResponse getFeatureFlagsResponse
		serverStatus   int
		wantFlags      map[string]bool
		wantErr        string
	}{
		{
			name:      "returns enabled flags",
			flagNames: []string{"gh_cli_telemetry"},
			serverResponse: getFeatureFlagsResponse{
				FeatureFlags: []featureFlag{
					{Name: "gh_cli_telemetry", IsEnabled: true},
				},
			},
			serverStatus: http.StatusOK,
			wantFlags:    map[string]bool{"gh_cli_telemetry": true},
		},
		{
			name:      "returns disabled flags",
			flagNames: []string{"gh_cli_telemetry"},
			serverResponse: getFeatureFlagsResponse{
				FeatureFlags: []featureFlag{
					{Name: "gh_cli_telemetry", IsEnabled: false},
				},
			},
			serverStatus: http.StatusOK,
			wantFlags:    map[string]bool{"gh_cli_telemetry": false},
		},
		{
			name:      "returns multiple flags",
			flagNames: []string{"flag_a", "flag_b"},
			serverResponse: getFeatureFlagsResponse{
				FeatureFlags: []featureFlag{
					{Name: "flag_a", IsEnabled: true},
					{Name: "flag_b", IsEnabled: false},
				},
			},
			serverStatus: http.StatusOK,
			wantFlags:    map[string]bool{"flag_a": true, "flag_b": false},
		},
		{
			name:         "returns error on non-200 status",
			flagNames:    []string{"gh_cli_telemetry"},
			serverStatus: http.StatusInternalServerError,
			wantErr:      "unexpected status 500 from CAFE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, getFeatureFlagsURL, r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var req getFeatureFlagsRequest
				require.NoError(t, json.Unmarshal(body, &req))
				assert.Equal(t, tt.flagNames, req.FlagNames)

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := NewClient(server.Client(), server.URL)
			flags, err := client.GetFeatureFlags(context.Background(), tt.flagNames)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantFlags, flags)
		})
	}
}

func TestGetFeatureFlags_connectionError(t *testing.T) {
	client := NewClient(http.DefaultClient, "http://localhost:1")
	_, err := client.GetFeatureFlags(context.Background(), []string{"flag"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "executing request")
}

func TestNewClient_defaultBaseURL(t *testing.T) {
	client := NewClient(http.DefaultClient, "")
	assert.Equal(t, defaultBaseURL, client.baseURL)
}
