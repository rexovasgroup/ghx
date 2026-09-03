// Package ghappauth implements short-lived GitHub App user authentication for
// ghx: a device-flow login and refresh-token rotation against a GitHub App,
// yielding expiring `ghu_` user access tokens instead of gh's long-lived OAuth
// token.
//
// Why a GitHub App (not gh's OAuth app): only GitHub Apps can issue expiring
// user tokens, and their device flow refreshes without a client secret — so the
// client ID below is safe to embed. Refreshing a token immediately invalidates
// the previous one server-side ("the old user access token will no longer
// work"), which is what makes rotation double as automatic revocation.
//
// The access token is stored through gh's normal auth config so every gh
// command uses it transparently; the refresh token + expiry are kept in a
// separate keyring entry (a "sidecar") managed only by this package.
package ghappauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/internal/keyring"
)

// DefaultClientID is the public "rexovas-gh-auth-token-oss" GitHub App client
// ID, embedded as the built-in default. It is non-secret by design: device flow
// requires no client secret. Users may override it (see ResolveClientID).
const DefaultClientID = "Iv23li4fpoAATTwwvEYZ"

// ClientIDEnvVar overrides the client ID via the environment (BYO app).
const ClientIDEnvVar = "GHX_AUTH_CLIENT_ID"

// defaultUserTokenLifetime is GitHub's fixed user-access-token lifetime, used
// only as a fallback when the token response omits expires_in.
const defaultUserTokenLifetime = 8 * time.Hour

// expiryLeeway makes callers refresh slightly early rather than racing the hard
// expiry boundary.
const expiryLeeway = 5 * time.Minute

// ResolveClientID picks the GitHub App client ID by precedence:
// flag value > GHX_AUTH_CLIENT_ID env var > config value > built-in default.
func ResolveClientID(flagValue, configValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv(ClientIDEnvVar); env != "" {
		return env
	}
	if configValue != "" {
		return configValue
	}
	return DefaultClientID
}

// Credentials is a short-lived user access token plus the material to rotate it.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	ClientID     string    `json:"client_id"`
}

// Expired reports whether the access token is at or near its expiry (within the
// leeway). A zero ExpiresAt is treated as expired so we err toward refreshing.
func (c Credentials) Expired() bool {
	return c.ExpiresAt.IsZero() || time.Now().After(c.ExpiresAt.Add(-expiryLeeway))
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error"`
	ErrorDesc       string `json:"error_description"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// webBase returns the OAuth web host for device/token endpoints. These live on
// the host itself (github.com/login/...), not the API subdomain.
func webBase(host string) string {
	if host == "" || host == "github.com" {
		return "https://github.com"
	}
	return "https://" + host
}

// DeviceFlow runs the GitHub App device flow and returns rotating credentials.
// It prints the one-time code and verification URL to out, then blocks polling
// until the user authorizes (or the device code expires / ctx is cancelled).
func DeviceFlow(ctx context.Context, httpClient *http.Client, host, clientID string, out io.Writer) (Credentials, error) {
	return deviceFlowAt(ctx, httpClient, webBase(host), clientID, out)
}

func deviceFlowAt(ctx context.Context, httpClient *http.Client, base, clientID string, out io.Writer) (Credentials, error) {
	dc, err := requestDeviceCode(ctx, httpClient, base, clientID)
	if err != nil {
		return Credentials{}, err
	}
	if dc.Error != "" {
		return Credentials{}, fmt.Errorf("device flow could not start: %s", firstNonEmpty(dc.ErrorDesc, dc.Error))
	}

	fmt.Fprintf(out, "! First copy your one-time code: %s\n", dc.UserCode)
	fmt.Fprintf(out, "Open this URL to continue in your web browser: %s\n", dc.VerificationURI)

	interval := dc.Interval
	if interval <= 0 {
		interval = 5
	}

	for {
		select {
		case <-ctx.Done():
			return Credentials{}, ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
		}

		tr, err := pollForToken(ctx, httpClient, base, clientID, dc.DeviceCode)
		if err != nil {
			return Credentials{}, err
		}
		switch tr.Error {
		case "":
			return credsFromToken(tr, clientID), nil
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			return Credentials{}, fmt.Errorf("the one-time code expired before authorization; run login again")
		case "access_denied":
			return Credentials{}, fmt.Errorf("authorization was denied")
		default:
			return Credentials{}, fmt.Errorf("device flow error: %s", firstNonEmpty(tr.ErrorDesc, tr.Error))
		}
	}
}

// Refresh exchanges a refresh token for a new access+refresh pair. Doing so
// invalidates the previous access token server-side.
func Refresh(ctx context.Context, httpClient *http.Client, host, clientID, refreshToken string) (Credentials, error) {
	return refreshAt(ctx, httpClient, webBase(host), clientID, refreshToken)
}

func refreshAt(ctx context.Context, httpClient *http.Client, base, clientID, refreshToken string) (Credentials, error) {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("grant_type", "refresh_token")
	v.Set("refresh_token", refreshToken)

	tr, err := postForm(ctx, httpClient, base+"/login/oauth/access_token", v)
	if err != nil {
		return Credentials{}, err
	}
	if tr.Error != "" {
		return Credentials{}, fmt.Errorf("token refresh failed (re-run login): %s", firstNonEmpty(tr.ErrorDesc, tr.Error))
	}
	return credsFromToken(tr, clientID), nil
}

func requestDeviceCode(ctx context.Context, httpClient *http.Client, base, clientID string) (deviceCodeResponse, error) {
	v := url.Values{}
	v.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/login/device/code", strings.NewReader(v.Encode()))
	if err != nil {
		return deviceCodeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return deviceCodeResponse{}, err
	}
	defer resp.Body.Close()

	var dc deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return deviceCodeResponse{}, fmt.Errorf("could not parse device code response: %w", err)
	}
	return dc, nil
}

func pollForToken(ctx context.Context, httpClient *http.Client, base, clientID, deviceCode string) (tokenResponse, error) {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("device_code", deviceCode)
	v.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	return postForm(ctx, httpClient, base+"/login/oauth/access_token", v)
}

func postForm(ctx context.Context, httpClient *http.Client, endpoint string, v url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return tokenResponse{}, fmt.Errorf("could not parse token response: %w", err)
	}
	return tr, nil
}

func credsFromToken(tr tokenResponse, clientID string) Credentials {
	lifetime := time.Duration(tr.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = defaultUserTokenLifetime
	}
	return Credentials{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(lifetime),
		ClientID:     clientID,
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// --- sidecar credential storage (keyring) ---

func keyringService(host string) string {
	return "ghx-app-auth:" + host
}

// StoreCredentials persists rotation material for (host, user) in the keyring,
// separate from gh's own token storage.
func StoreCredentials(host, user string, c Credentials) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService(host), user, string(b))
}

// LoadCredentials returns the stored rotation material for (host, user). It
// returns an error if none is stored — which is the normal case for accounts
// not logged in via the GitHub App flow, so callers treat that as "not an app
// account" rather than a failure.
func LoadCredentials(host, user string) (Credentials, error) {
	s, err := keyring.Get(keyringService(host), user)
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return Credentials{}, fmt.Errorf("stored GitHub App credentials are corrupt: %w", err)
	}
	return c, nil
}

// DeleteCredentials removes the sidecar rotation material for (host, user).
func DeleteCredentials(host, user string) error {
	return keyring.Delete(keyringService(host), user)
}
