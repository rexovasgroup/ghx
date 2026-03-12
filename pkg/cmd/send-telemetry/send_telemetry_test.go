package sendtelemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCmdSendTelemetry(t *testing.T) {
	tests := []struct {
		name     string
		stdin    string
		env      map[string]string
		wantOpts SendTelemetryOptions
	}{
		{
			name:  "reads payload from stdin",
			stdin: `{"eventType":"usage","dimensions":{"command":"gh pr list"}}`,
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL:     defaultCentralEndpointURL,
				FeatureFlagEndpointURL: defaultFeatureFlagEndpointURL,
				PayloadJSON:            `{"eventType":"usage","dimensions":{"command":"gh pr list"}}`,
			},
		},
		{
			name:  "uses CENTRAL_ENDPOINT_URL env var",
			stdin: `{"eventType":"usage"}`,
			env:   map[string]string{"CENTRAL_ENDPOINT_URL": "https://custom.endpoint/api"},
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL:     "https://custom.endpoint/api",
				FeatureFlagEndpointURL: defaultFeatureFlagEndpointURL,
				PayloadJSON:            `{"eventType":"usage"}`,
			},
		},
		{
			name:  "uses FEATURE_FLAG_ENDPOINT_URL env var",
			stdin: `{}`,
			env:   map[string]string{"FEATURE_FLAG_ENDPOINT_URL": "https://custom.cafe/api"},
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL:     defaultCentralEndpointURL,
				FeatureFlagEndpointURL: "https://custom.cafe/api",
				PayloadJSON:            `{}`,
			},
		},
		{
			name:  "defaults endpoint when env var not set",
			stdin: `{}`,
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL:     defaultCentralEndpointURL,
				FeatureFlagEndpointURL: defaultFeatureFlagEndpointURL,
				PayloadJSON:            `{}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			ios, stdin, _, _ := iostreams.Test()
			stdin.WriteString(tt.stdin)
			f := &cmdutil.Factory{
				IOStreams: ios,
				Config: func() (gh.Config, error) {
					return config.NewBlankConfig(), nil
				},
			}

			var gotOpts *SendTelemetryOptions
			cmd := newCmdSendTelemetry(f, func(opts *SendTelemetryOptions) error {
				gotOpts = opts
				return nil
			})
			cmd.SetArgs([]string{})
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err := cmd.ExecuteC()
			require.NoError(t, err)
			require.NotNil(t, gotOpts)
			assert.Equal(t, tt.wantOpts.CentralEndpointURL, gotOpts.CentralEndpointURL)
			assert.Equal(t, tt.wantOpts.FeatureFlagEndpointURL, gotOpts.FeatureFlagEndpointURL)
			assert.Equal(t, tt.wantOpts.PayloadJSON, gotOpts.PayloadJSON)
			assert.Equal(t, tt.wantOpts.HTTPUnixSocket, gotOpts.HTTPUnixSocket)
		})
	}
}

func TestRunSendTelemetry(t *testing.T) {
	// Helper to create a CAFE server that returns the flag as enabled
	cafeEnabledServer := func(t *testing.T) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"feature_flags": []map[string]any{
					{"name": "gh_cli_telemetry", "is_enabled": true},
				},
			})
		}))
	}

	tests := []struct {
		name       string
		opts       *SendTelemetryOptions
		handler    http.HandlerFunc
		wantErr    bool
		setupCAFE  bool
		assertFunc func(t *testing.T, receivedBody []byte, receivedContentType string, receivedPath string)
	}{
		{
			name: "posts event to endpoint",
			opts: &SendTelemetryOptions{
				PayloadJSON: mustMarshal(t, &telemetry.Event{
					EventType: "usage",
					Dimensions: telemetry.Dimensions{
						Command:      "gh pr create",
						DeviceID:     "abc123hashed",
						OS:           "darwin",
						Architecture: "arm64",
						Version:      "2.45.0",
					},
				}),
				AuthToken: "test-token",
			},
			setupCAFE: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			assertFunc: func(t *testing.T, receivedBody []byte, receivedContentType string, receivedPath string) {
				t.Helper()
				assert.Equal(t, "/api/usage/github-cli", receivedPath)
				assert.Equal(t, "application/json", receivedContentType)

				var received telemetry.Event
				require.NoError(t, json.Unmarshal(receivedBody, &received))
				assert.Equal(t, "usage", received.EventType)
				assert.Equal(t, "gh pr create", received.Dimensions.Command)
				assert.Equal(t, "abc123hashed", received.Dimensions.DeviceID)
			},
		},
		{
			name: "invalid JSON is silently ignored",
			opts: &SendTelemetryOptions{
				PayloadJSON: "not valid json",
			},
		},
		{
			name: "server error is silently ignored",
			opts: &SendTelemetryOptions{
				PayloadJSON: mustMarshal(t, &telemetry.Event{
					EventType: "usage",
					Dimensions: telemetry.Dimensions{
						Command: "gh pr list",
					},
				}),
				AuthToken: "test-token",
			},
			setupCAFE: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.StateDir = t.TempDir()

			if tt.setupCAFE {
				cafe := cafeEnabledServer(t)
				defer cafe.Close()
				tt.opts.FeatureFlagEndpointURL = cafe.URL
			}

			var receivedBody []byte
			var receivedContentType string
			var receivedPath string

			if tt.handler != nil {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedPath = r.URL.Path
					receivedContentType = r.Header.Get("Content-Type")
					var err error
					receivedBody, err = io.ReadAll(r.Body)
					require.NoError(t, err)
					tt.handler(w, r)
				}))
				defer server.Close()
				tt.opts.CentralEndpointURL = server.URL + "/api/usage/github-cli"
			}

			err := runSendTelemetry(tt.opts)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.assertFunc != nil {
				tt.assertFunc(t, receivedBody, receivedContentType, receivedPath)
			}
		})
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func TestIsTelemetryFlagEnabled(t *testing.T) {
	tests := []struct {
		name         string
		opts         *SendTelemetryOptions
		cafeHandler  http.HandlerFunc
		wantEnabled  bool
	}{
		{
			name: "enterprise host returns false",
			opts: &SendTelemetryOptions{
				IsEnterprise: true,
				AuthToken:    "token123",
			},
			wantEnabled: false,
		},
		{
			name: "no auth token fails closed",
			opts: &SendTelemetryOptions{
				IsEnterprise: false,
				AuthToken:    "",
			},
			wantEnabled: false,
		},
		{
			name: "flag enabled returns true",
			opts: &SendTelemetryOptions{
				IsEnterprise: false,
				AuthToken:    "token123",
			},
			cafeHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
				json.NewEncoder(w).Encode(map[string]any{
					"feature_flags": []map[string]any{
						{"name": "gh_cli_telemetry", "is_enabled": true},
					},
				})
			},
			wantEnabled: true,
		},
		{
			name: "flag disabled returns false",
			opts: &SendTelemetryOptions{
				IsEnterprise: false,
				AuthToken:    "token123",
			},
			cafeHandler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{
					"feature_flags": []map[string]any{
						{"name": "gh_cli_telemetry", "is_enabled": false},
					},
				})
			},
			wantEnabled: false,
		},
		{
			name: "CAFE error fails closed",
			opts: &SendTelemetryOptions{
				IsEnterprise: false,
				AuthToken:    "token123",
			},
			cafeHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantEnabled: false,
		},
		{
			name: "flag missing from response fails closed",
			opts: &SendTelemetryOptions{
				IsEnterprise: false,
				AuthToken:    "token123",
			},
			cafeHandler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{
					"feature_flags": []map[string]any{},
				})
			},
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.StateDir = t.TempDir()

			if tt.cafeHandler != nil {
				server := httptest.NewServer(tt.cafeHandler)
				defer server.Close()
				tt.opts.FeatureFlagEndpointURL = server.URL
			} else if !tt.opts.IsEnterprise && tt.opts.AuthToken == "" {
				// No server needed for these cases
			} else if !tt.opts.IsEnterprise {
				tt.opts.FeatureFlagEndpointURL = "http://localhost:1"
			}

			got := isTelemetryFlagEnabled(tt.opts)
			assert.Equal(t, tt.wantEnabled, got)
		})
	}
}

func TestRunSendTelemetry_featureFlagGating(t *testing.T) {
	validPayload := mustMarshal(t, &telemetry.Event{
		EventType: "usage",
		Dimensions: telemetry.Dimensions{
			Command: "gh pr list",
		},
	})

	t.Run("sends telemetry when flag enabled", func(t *testing.T) {
		var centralReceived bool
		centralServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			centralReceived = true
			w.WriteHeader(http.StatusOK)
		}))
		defer centralServer.Close()

		cafeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"feature_flags": []map[string]any{
					{"name": "gh_cli_telemetry", "is_enabled": true},
				},
			})
		}))
		defer cafeServer.Close()

		opts := &SendTelemetryOptions{
			CentralEndpointURL:     centralServer.URL + "/api/usage/github-cli",
			FeatureFlagEndpointURL: cafeServer.URL,
			PayloadJSON:            validPayload,
			AuthToken:              "token123",
			StateDir:               t.TempDir(),
		}

		err := runSendTelemetry(opts)
		require.NoError(t, err)
		assert.True(t, centralReceived, "expected telemetry to be sent to Central")
	})

	t.Run("skips telemetry when flag disabled", func(t *testing.T) {
		var centralReceived bool
		centralServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			centralReceived = true
			w.WriteHeader(http.StatusOK)
		}))
		defer centralServer.Close()

		cafeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"feature_flags": []map[string]any{
					{"name": "gh_cli_telemetry", "is_enabled": false},
				},
			})
		}))
		defer cafeServer.Close()

		opts := &SendTelemetryOptions{
			CentralEndpointURL:     centralServer.URL + "/api/usage/github-cli",
			FeatureFlagEndpointURL: cafeServer.URL,
			PayloadJSON:            validPayload,
			AuthToken:              "token123",
			StateDir:               t.TempDir(),
		}

		err := runSendTelemetry(opts)
		require.NoError(t, err)
		assert.False(t, centralReceived, "expected telemetry NOT to be sent to Central")
	})

	t.Run("skips telemetry for enterprise host", func(t *testing.T) {
		var centralReceived bool
		centralServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			centralReceived = true
			w.WriteHeader(http.StatusOK)
		}))
		defer centralServer.Close()

		opts := &SendTelemetryOptions{
			CentralEndpointURL: centralServer.URL + "/api/usage/github-cli",
			PayloadJSON:        validPayload,
			IsEnterprise:       true,
			StateDir:           t.TempDir(),
		}

		err := runSendTelemetry(opts)
		require.NoError(t, err)
		assert.False(t, centralReceived, "expected telemetry NOT to be sent for enterprise host")
	})
}
