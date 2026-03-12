package sendtelemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/internal/featureflags/cafe"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeViewerAPI implements cafe.ViewerAPI for test CAFE servers.
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

func newCAFEServer(t *testing.T, api cafe.ViewerAPI) *httptest.Server {
	t.Helper()
	handler := cafe.NewViewerAPIServer(api)
	mux := http.NewServeMux()
	mux.Handle(handler.PathPrefix(), handler)
	return httptest.NewServer(mux)
}

func newCAFEServerWithFlags(t *testing.T, flags ...*cafe.FeatureFlag) *httptest.Server {
	t.Helper()
	return newCAFEServer(t, &fakeViewerAPI{flags: flags})
}

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
		return newCAFEServerWithFlags(t, &cafe.FeatureFlag{Name: telemetryFeatureFlag, IsEnabled: true})
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
				HostType:  telemetry.HostTypeDotcom,
				Host:      "github.com",
				User:      "testuser",
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
				HostType:  telemetry.HostTypeDotcom,
				Host:      "github.com",
				User:      "testuser",
			},
			setupCAFE: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.CacheDir = t.TempDir()

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
		name        string
		opts        *SendTelemetryOptions
		cafeServer  func(t *testing.T) *httptest.Server
		wantEnabled bool
	}{
		{
			name: "enterprise host returns false",
			opts: &SendTelemetryOptions{
				HostType:  telemetry.HostTypeEnterprise,
				AuthToken: "token123",
			},
			wantEnabled: false,
		},
		{
			name: "no auth token fails closed",
			opts: &SendTelemetryOptions{
				HostType:  telemetry.HostTypeDotcom,
				AuthToken: "",
			},
			wantEnabled: false,
		},
		{
			name: "flag enabled returns true",
			opts: &SendTelemetryOptions{
				HostType:  telemetry.HostTypeDotcom,
				AuthToken: "token123",
			},
			cafeServer: func(t *testing.T) *httptest.Server {
				return newCAFEServerWithFlags(t, &cafe.FeatureFlag{Name: telemetryFeatureFlag, IsEnabled: true})
			},
			wantEnabled: true,
		},
		{
			name: "flag disabled returns false",
			opts: &SendTelemetryOptions{
				HostType:  telemetry.HostTypeDotcom,
				AuthToken: "token123",
			},
			cafeServer: func(t *testing.T) *httptest.Server {
				return newCAFEServerWithFlags(t, &cafe.FeatureFlag{Name: telemetryFeatureFlag, IsEnabled: false})
			},
			wantEnabled: false,
		},
		{
			name: "CAFE error fails closed",
			opts: &SendTelemetryOptions{
				HostType:  telemetry.HostTypeDotcom,
				AuthToken: "token123",
			},
			cafeServer: func(t *testing.T) *httptest.Server {
				return newCAFEServer(t, &fakeViewerAPI{stubbedErr: assert.AnError})
			},
			wantEnabled: false,
		},
		{
			name: "flag missing from response fails closed",
			opts: &SendTelemetryOptions{
				HostType:  telemetry.HostTypeDotcom,
				AuthToken: "token123",
			},
			cafeServer: func(t *testing.T) *httptest.Server {
				return newCAFEServerWithFlags(t)
			},
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.CacheDir = t.TempDir()
			if tt.opts.Host == "" {
				tt.opts.Host = "github.com"
			}
			if tt.opts.User == "" {
				tt.opts.User = "testuser"
			}

			if tt.cafeServer != nil {
				server := tt.cafeServer(t)
				defer server.Close()
				tt.opts.FeatureFlagEndpointURL = server.URL
			} else if tt.opts.HostType == telemetry.HostTypeDotcom && tt.opts.AuthToken == "" {
				// No server needed for these cases
			} else if tt.opts.HostType == telemetry.HostTypeDotcom {
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

		cafeServer := newCAFEServerWithFlags(t, &cafe.FeatureFlag{Name: telemetryFeatureFlag, IsEnabled: true})
		defer cafeServer.Close()

		opts := &SendTelemetryOptions{
			CentralEndpointURL:     centralServer.URL + "/api/usage/github-cli",
			FeatureFlagEndpointURL: cafeServer.URL,
			PayloadJSON:            validPayload,
			AuthToken:              "token123",
			CacheDir:               t.TempDir(),
			Host:                   "github.com",
			User:                   "testuser",
			HostType:               telemetry.HostTypeDotcom,
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

		cafeServer := newCAFEServerWithFlags(t, &cafe.FeatureFlag{Name: telemetryFeatureFlag, IsEnabled: false})
		defer cafeServer.Close()

		opts := &SendTelemetryOptions{
			CentralEndpointURL:     centralServer.URL + "/api/usage/github-cli",
			FeatureFlagEndpointURL: cafeServer.URL,
			PayloadJSON:            validPayload,
			AuthToken:              "token123",
			CacheDir:               t.TempDir(),
			Host:                   "github.com",
			User:                   "testuser",
			HostType:               telemetry.HostTypeDotcom,
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
			HostType:           telemetry.HostTypeEnterprise,
			CacheDir:           t.TempDir(),
		}

		err := runSendTelemetry(opts)
		require.NoError(t, err)
		assert.False(t, centralReceived, "expected telemetry NOT to be sent for enterprise host")
	})
}
