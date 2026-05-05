package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/internal/gh/ghtelemetry"
	"github.com/cli/cli/v2/utils"
	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	ghauth "github.com/cli/go-gh/v2/pkg/auth"
)

// AggressiveCachingTTLEnv is the environment variable that opts the standard
// HTTP client into process-wide TTL caching for cacheable REST + GraphQL
// requests. The value is a Go duration string (e.g. "30s", "1m", "5m", "1h").
// Anything that does not parse to a positive duration silently disables the
// feature; users diagnose by inspecting the X-GH-Cache-Status response header
// (visible via gh api -i or GH_DEBUG=api).
//
// This is intended for automation/agent harnesses that issue repeated identical
// requests, not for general interactive use. Reads may be up to TTL stale,
// including reads following a write from the same or another process.
const AggressiveCachingTTLEnv = "GH_AGGRESSIVE_CACHING_TTL"

type tokenGetter interface {
	ActiveToken(string) (string, string)
}

type HTTPClientOptions struct {
	AppVersion         string
	InvokingAgent      string
	CacheTTL           time.Duration
	Config             tokenGetter
	EnableCache        bool
	Log                io.Writer
	LogColorize        bool
	LogVerboseHTTP     bool
	SkipDefaultHeaders bool
	TelemetryDisabler  ghtelemetry.Disabler
}

func NewHTTPClient(opts HTTPClientOptions) (*http.Client, error) {
	// Provide invalid host, and token values so gh.HTTPClient will not automatically resolve them.
	// The real host and token are inserted at request time.
	clientOpts := ghAPI.ClientOptions{
		Host:               "none",
		AuthToken:          "none",
		LogIgnoreEnv:       true,
		SkipDefaultHeaders: opts.SkipDefaultHeaders,
	}

	debugEnabled, debugValue := utils.IsDebugEnabled()
	if strings.Contains(debugValue, "api") {
		opts.LogVerboseHTTP = true
	}

	if opts.LogVerboseHTTP || debugEnabled {
		clientOpts.Log = opts.Log
		clientOpts.LogColorize = opts.LogColorize
		clientOpts.LogVerboseHTTP = opts.LogVerboseHTTP
	}

	ua := fmt.Sprintf("GitHub CLI %s", opts.AppVersion)
	if opts.InvokingAgent != "" {
		ua = fmt.Sprintf("%s Agent/%s", ua, opts.InvokingAgent)
	}

	headers := map[string]string{
		userAgent:  ua,
		apiVersion: apiVersionValue,
	}
	clientOpts.Headers = headers

	if opts.EnableCache {
		clientOpts.EnableCache = opts.EnableCache
		clientOpts.CacheTTL = opts.CacheTTL
	} else if !opts.SkipDefaultHeaders {
		// Opt into process-wide aggressive caching when GH_AGGRESSIVE_CACHING_TTL
		// is set to a positive Go duration. Two gates apply:
		//
		//   1. opts.EnableCache must not already be set: explicit caller
		//      configuration (e.g. gh api --cache 5m) wins over the env var.
		//   2. opts.SkipDefaultHeaders must be false: this distinguishes the
		//      standard authenticated client from PlainHttpClient, which is
		//      built specifically to bypass defaults and must not start caching
		//      just because an env var is set.
		//
		// Per-request opt-out via X-GH-CACHE-TTL: 0 still works on top of this
		// (handled in the underlying go-gh cache transport).
		if ttl, ok := parseAggressiveCachingTTL(os.Getenv(AggressiveCachingTTLEnv)); ok {
			clientOpts.EnableCache = true
			clientOpts.CacheTTL = ttl
		}
	}

	client, err := ghAPI.NewHTTPClient(clientOpts)
	if err != nil {
		return nil, err
	}

	if opts.Config != nil {
		client.Transport = AddAuthTokenHeader(client.Transport, opts.Config)
	}

	if opts.TelemetryDisabler != nil {
		client.Transport = telemetryDisablerTransport{
			wrappedTransport:  client.Transport,
			telemetryDisabler: opts.TelemetryDisabler,
		}
	}

	return client, nil
}

// parseAggressiveCachingTTL parses the GH_AGGRESSIVE_CACHING_TTL value. It
// returns (ttl, true) only when the value is a Go duration string that parses
// to a strictly positive value. Anything else (empty, "0", "0s", unparseable)
// returns (0, false), which keeps the cache off.
//
// Failures are silent by design: this matches the convention for other GH_*
// env vars (we do not have a stable warning channel inside NewHTTPClient and
// stderr noise from a transport layer would surprise scripts). Users who
// believe the variable should be active can confirm it via the
// X-GH-Cache-Status response header on subsequent requests.
func parseAggressiveCachingTTL(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, false
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false
	}
	if d <= 0 {
		return 0, false
	}
	return d, true
}

func NewCachedHTTPClient(httpClient *http.Client, ttl time.Duration) *http.Client {
	newClient := *httpClient
	newClient.Transport = AddCacheTTLHeader(httpClient.Transport, ttl)
	return &newClient
}

// AddCacheTTLHeader adds an header to the request telling the cache that the request
// should be cached for a specified amount of time.
func AddCacheTTLHeader(rt http.RoundTripper, ttl time.Duration) http.RoundTripper {
	return &funcTripper{roundTrip: func(req *http.Request) (*http.Response, error) {
		// If the header is already set in the request, don't overwrite it.
		if req.Header.Get(cacheTTL) == "" {
			req.Header.Set(cacheTTL, ttl.String())
		}
		return rt.RoundTrip(req)
	}}
}

// AddAuthTokenHeader adds an authentication token header for the host specified by the request.
func AddAuthTokenHeader(rt http.RoundTripper, cfg tokenGetter) http.RoundTripper {
	return &funcTripper{roundTrip: func(req *http.Request) (*http.Response, error) {
		// If the header is already set in the request, don't overwrite it.
		if req.Header.Get(authorization) == "" {
			var redirectHostnameChange bool
			if req.Response != nil && req.Response.Request != nil {
				redirectHostnameChange = getHost(req) != getHost(req.Response.Request)
			}
			// Only set header if an initial request or redirect request to the same host as the initial request.
			// If the host has changed during a redirect do not add the authentication token header.
			if !redirectHostnameChange {
				hostname := ghauth.NormalizeHostname(getHost(req))
				if token, _ := cfg.ActiveToken(hostname); token != "" {
					req.Header.Set(authorization, fmt.Sprintf("token %s", token))
				}
			}
		}
		return rt.RoundTrip(req)
	}}
}

// ExtractHeader extracts a named header from any response received by this client and,
// if non-blank, saves it to dest.
func ExtractHeader(name string, dest *string) func(http.RoundTripper) http.RoundTripper {
	return func(tr http.RoundTripper) http.RoundTripper {
		return &funcTripper{roundTrip: func(req *http.Request) (*http.Response, error) {
			res, err := tr.RoundTrip(req)
			if err == nil {
				if value := res.Header.Get(name); value != "" {
					*dest = value
				}
			}
			return res, err
		}}
	}
}

type funcTripper struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (tr funcTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return tr.roundTrip(req)
}

func getHost(r *http.Request) string {
	if r.Host != "" {
		return r.Host
	}
	return r.URL.Host
}

type telemetryDisablerTransport struct {
	wrappedTransport  http.RoundTripper
	telemetryDisabler ghtelemetry.Disabler
}

func (t telemetryDisablerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if ghauth.IsEnterprise(getHost(req)) {
		t.telemetryDisabler.Disable()
	}
	return t.wrappedTransport.RoundTrip(req)
}
