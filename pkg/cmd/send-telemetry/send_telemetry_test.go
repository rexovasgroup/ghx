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
		payload  string
		env      map[string]string
		wantOpts SendTelemetryOptions
	}{
		{
			// Given a telemetry payload as an argument
			// When I run send-telemetry
			// Then the payload is passed through to the options
			name:    "reads payload from argument",
			payload: `{"eventType":"usage","dimensions":{"command":"gh pr list"}}`,
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL: defaultCentralEndpointURL,
				PayloadJSON:        `{"eventType":"usage","dimensions":{"command":"gh pr list"}}`,
			},
		},
		{
			// Given CENTRAL_ENDPOINT_URL is set
			// When I run send-telemetry
			// Then the custom endpoint is used
			name:    "uses CENTRAL_ENDPOINT_URL env var",
			payload: `{"eventType":"usage"}`,
			env:     map[string]string{"CENTRAL_ENDPOINT_URL": "https://custom.endpoint/api"},
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL: "https://custom.endpoint/api",
				PayloadJSON:        `{"eventType":"usage"}`,
			},
		},
		{
			// Given no CENTRAL_ENDPOINT_URL is set
			// When I run send-telemetry
			// Then the default endpoint is used
			name:    "defaults endpoint when env var not set",
			payload: `{}`,
			wantOpts: SendTelemetryOptions{
				CentralEndpointURL: defaultCentralEndpointURL,
				PayloadJSON:        `{}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			ios, _, _, _ := iostreams.Test()
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
			cmd.SetArgs([]string{tt.payload})
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err := cmd.ExecuteC()
			require.NoError(t, err)
			require.NotNil(t, gotOpts)
			assert.Equal(t, tt.wantOpts.CentralEndpointURL, gotOpts.CentralEndpointURL)
			assert.Equal(t, tt.wantOpts.PayloadJSON, gotOpts.PayloadJSON)
			assert.Equal(t, tt.wantOpts.HTTPUnixSocket, gotOpts.HTTPUnixSocket)
		})
	}
}

func TestRunSendTelemetry(t *testing.T) {
	tests := []struct {
		name       string
		opts       *SendTelemetryOptions
		handler    http.HandlerFunc
		wantErr    bool
		assertFunc func(t *testing.T, receivedBody []byte, receivedContentType string, receivedPath string)
	}{
		{
			// Given a valid telemetry payload
			// When I send telemetry
			// Then it should POST to the endpoint with the correct content type
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
			},
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
			// Given a server that returns an error
			// When I send telemetry
			// Then the error should be silently ignored
			name: "server error is silently ignored",
			opts: &SendTelemetryOptions{
				PayloadJSON: mustMarshal(t, &telemetry.Event{
					EventType: "usage",
					Dimensions: telemetry.Dimensions{
						Command: "gh pr list",
					},
				}),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
