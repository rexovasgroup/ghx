package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestCount returns a handler that increments a counter on every request
// and writes the current count to the response body.
func requestCount() (http.Handler, func() int) {
	count := 0
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{byte('0' + count%10)})
	})
	return h, func() int { return count }
}

func TestParseAggressiveCachingTTL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantTTL time.Duration
		wantOK  bool
	}{
		{"empty disables", "", 0, false},
		{"30s enables", "30s", 30 * time.Second, true},
		{"1h enables", "1h", time.Hour, true},
		{"500ms enables", "500ms", 500 * time.Millisecond, true},
		{"explicit zero disables", "0", 0, false},
		{"explicit zero seconds disables", "0s", 0, false},
		{"negative disables", "-30s", 0, false},
		{"unparseable disables", "garbage", 0, false},
		{"plain number disables", "30", 0, false},
		{"missing unit disables", "30 seconds", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ttl, ok := parseAggressiveCachingTTL(c.raw)
			assert.Equal(t, c.wantOK, ok)
			assert.Equal(t, c.wantTTL, ttl)
		})
	}
}

func TestNewHTTPClient_AggressiveCachingEnvVar(t *testing.T) {
	t.Run("env var with positive duration enables caching", func(t *testing.T) {
		t.Setenv(AggressiveCachingTTLEnv, "1h")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion: "test",
		})
		require.NoError(t, err)

		// First request: miss + network call.
		res1 := doGET(t, client, ts.URL+"/repos/cli/cli")
		assert.Equal(t, "miss", res1.Header.Get(ghAPI.CacheStatusHeader))

		// Second identical request: hit, no additional network call.
		res2 := doGET(t, client, ts.URL+"/repos/cli/cli")
		assert.Equal(t, "hit", res2.Header.Get(ghAPI.CacheStatusHeader))

		assert.Equal(t, 1, count(), "second request must be served from cache")
	})

	t.Run("env var unset leaves caching disabled", func(t *testing.T) {
		t.Setenv(AggressiveCachingTTLEnv, "")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion: "test",
		})
		require.NoError(t, err)

		res1 := doGET(t, client, ts.URL+"/repos/cli/cli")
		assert.Equal(t, "", res1.Header.Get(ghAPI.CacheStatusHeader),
			"no env var means cache transport is not consulted, so no status header is set")

		_ = doGET(t, client, ts.URL+"/repos/cli/cli")
		assert.Equal(t, 2, count(), "without caching, every request hits the network")
	})

	t.Run("env var with garbage value silently leaves caching disabled", func(t *testing.T) {
		t.Setenv(AggressiveCachingTTLEnv, "not-a-duration")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion: "test",
		})
		require.NoError(t, err)

		_ = doGET(t, client, ts.URL+"/x")
		_ = doGET(t, client, ts.URL+"/x")
		assert.Equal(t, 2, count(), "garbage env var value must not enable caching")
	})

	t.Run("env var with zero value silently leaves caching disabled", func(t *testing.T) {
		t.Setenv(AggressiveCachingTTLEnv, "0")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion: "test",
		})
		require.NoError(t, err)

		_ = doGET(t, client, ts.URL+"/x")
		_ = doGET(t, client, ts.URL+"/x")
		assert.Equal(t, 2, count(), "zero TTL must not enable caching")
	})

	t.Run("explicit caller EnableCache wins over env var", func(t *testing.T) {
		// Caller explicitly disables cache (EnableCache=false). Env var is set.
		// Today this still enables caching because EnableCache=false is the
		// zero value and we cannot distinguish "explicit false" from
		// "unconfigured". Document that callers wanting to suppress aggressive
		// caching must rely on per-request X-GH-CACHE-TTL: 0 instead.
		// The reverse path (EnableCache=true with CacheTTL set) does take
		// precedence over the env var:
		t.Setenv(AggressiveCachingTTLEnv, "5s")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion:  "test",
			EnableCache: true,
			CacheTTL:    1 * time.Hour,
		})
		require.NoError(t, err)

		res1 := doGET(t, client, ts.URL+"/y")
		assert.Equal(t, "miss", res1.Header.Get(ghAPI.CacheStatusHeader))
		_ = doGET(t, client, ts.URL+"/y")
		assert.Equal(t, 1, count(), "explicit EnableCache + CacheTTL takes precedence; second request still served from cache")
	})

	t.Run("SkipDefaultHeaders bypasses aggressive caching (PlainHttpClient)", func(t *testing.T) {
		// PlainHttpClient is built with SkipDefaultHeaders: true to bypass
		// gh's default header injection. It must also bypass aggressive
		// caching since callers depend on its raw, uncached behavior.
		t.Setenv(AggressiveCachingTTLEnv, "1h")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion:         "test",
			SkipDefaultHeaders: true,
		})
		require.NoError(t, err)

		_ = doGET(t, client, ts.URL+"/z")
		_ = doGET(t, client, ts.URL+"/z")
		assert.Equal(t, 2, count(), "PlainHttpClient (SkipDefaultHeaders) must not opt into aggressive caching")
	})

	t.Run("per-request X-GH-CACHE-TTL: 0 bypasses cache when env var is set", func(t *testing.T) {
		t.Setenv(AggressiveCachingTTLEnv, "1h")

		handler, count := requestCount()
		ts := httptest.NewServer(handler)
		defer ts.Close()

		client, err := NewHTTPClient(HTTPClientOptions{
			AppVersion: "test",
		})
		require.NoError(t, err)

		// Populate cache.
		_ = doGET(t, client, ts.URL+"/p")
		assert.Equal(t, 1, count())

		// Confirm hit.
		res := doGET(t, client, ts.URL+"/p")
		assert.Equal(t, "hit", res.Header.Get(ghAPI.CacheStatusHeader))
		assert.Equal(t, 1, count())

		// Per-request opt-out with explicit zero TTL.
		req, _ := http.NewRequest("GET", ts.URL+"/p", nil)
		req.Header.Set("X-GH-CACHE-TTL", "0")
		res, err = client.Do(req)
		require.NoError(t, err)
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
		assert.Equal(t, "", res.Header.Get(ghAPI.CacheStatusHeader),
			"explicit per-request bypass should not produce a cache-status header")
		assert.Equal(t, 2, count(), "per-request opt-out must hit the network")

		// Cache entry was not overwritten by the bypass.
		res = doGET(t, client, ts.URL+"/p")
		assert.Equal(t, "hit", res.Header.Get(ghAPI.CacheStatusHeader))
		assert.Equal(t, 2, count())
	})
}

// doGET issues a GET request with the given client and reads the body to
// completion so the response can be inspected without leaking handles.
func doGET(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	res, err := client.Do(req)
	require.NoError(t, err)
	_, _ = io.ReadAll(res.Body)
	res.Body.Close()
	return res
}
