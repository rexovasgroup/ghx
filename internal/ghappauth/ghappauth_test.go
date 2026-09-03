package ghappauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cli/cli/v2/internal/keyring"
)

func TestResolveClientID(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		env    string
		config string
		want   string
	}{
		{name: "default when nothing set", want: DefaultClientID},
		{name: "config over default", config: "cfg123", want: "cfg123"},
		{name: "env over config", env: "env123", config: "cfg123", want: "env123"},
		{name: "flag over everything", flag: "flag123", env: "env123", config: "cfg123", want: "flag123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv(ClientIDEnvVar, tt.env)
			} else {
				t.Setenv(ClientIDEnvVar, "")
			}
			if got := ResolveClientID(tt.flag, tt.config); got != tt.want {
				t.Errorf("ResolveClientID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCredentialsExpired(t *testing.T) {
	if !(Credentials{}).Expired() {
		t.Error("zero ExpiresAt should be treated as expired")
	}
	if !(Credentials{ExpiresAt: time.Now().Add(1 * time.Minute)}).Expired() {
		t.Error("token within the leeway window should be considered expired")
	}
	if (Credentials{ExpiresAt: time.Now().Add(1 * time.Hour)}).Expired() {
		t.Error("token an hour out should not be expired")
	}
}

func TestWebBase(t *testing.T) {
	cases := map[string]string{
		"":                    "https://github.com",
		"github.com":          "https://github.com",
		"enterprise.internal": "https://enterprise.internal",
	}
	for host, want := range cases {
		if got := webBase(host); got != want {
			t.Errorf("webBase(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestCredsFromToken(t *testing.T) {
	c := credsFromToken(tokenResponse{AccessToken: "ghu_x", RefreshToken: "ghr_y", ExpiresIn: 28800}, "cid")
	if c.AccessToken != "ghu_x" || c.RefreshToken != "ghr_y" || c.ClientID != "cid" {
		t.Fatalf("unexpected creds: %+v", c)
	}
	if d := time.Until(c.ExpiresAt); d < 7*time.Hour || d > 9*time.Hour {
		t.Errorf("expiry from expires_in=28800 should be ~8h, got %s", d)
	}

	// Missing expires_in falls back to the fixed 8h lifetime.
	c2 := credsFromToken(tokenResponse{AccessToken: "ghu_z"}, "cid")
	if d := time.Until(c2.ExpiresAt); d < 7*time.Hour || d > 9*time.Hour {
		t.Errorf("fallback expiry should be ~8h, got %s", d)
	}
}

func TestRefresh(t *testing.T) {
	var gotForm string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.Form.Encode()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ghu_new","refresh_token":"ghr_new","expires_in":28800}`))
	}))
	defer srv.Close()

	c, err := refreshAt(context.Background(), srv.Client(), srv.URL, "cid", "ghr_old")
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "ghu_new" || c.RefreshToken != "ghr_new" {
		t.Fatalf("unexpected creds: %+v", c)
	}
	if !strings.Contains(gotForm, "grant_type=refresh_token") || !strings.Contains(gotForm, "refresh_token=ghr_old") {
		t.Errorf("unexpected refresh form: %s", gotForm)
	}
}

func TestRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"bad_refresh_token","error_description":"expired"}`))
	}))
	defer srv.Close()

	if _, err := refreshAt(context.Background(), srv.Client(), srv.URL, "cid", "ghr_old"); err == nil {
		t.Fatal("expected an error for a bad refresh token response")
	}
}

func TestDeviceFlowPolling(t *testing.T) {
	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/device/code":
			_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","interval":1}`))
		case "/login/oauth/access_token":
			polls++
			if polls < 2 {
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"ghu_ok","refresh_token":"ghr_ok","expires_in":28800}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	var out strings.Builder
	c, err := deviceFlowAt(context.Background(), srv.Client(), srv.URL, "cid", &out)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "ghu_ok" || c.RefreshToken != "ghr_ok" {
		t.Fatalf("unexpected creds: %+v", c)
	}
	if polls < 2 {
		t.Errorf("expected to poll through authorization_pending, polls=%d", polls)
	}
	if !strings.Contains(out.String(), "ABCD-1234") {
		t.Errorf("expected the user code to be shown, got: %q", out.String())
	}
}

func TestStoreLoadDeleteCredentials(t *testing.T) {
	keyring.MockInit()

	want := Credentials{AccessToken: "ghu_x", RefreshToken: "ghr_y", ExpiresAt: time.Now().Add(time.Hour).Round(time.Second), ClientID: "cid"}
	if err := StoreCredentials("github.com", "octocat", want); err != nil {
		t.Fatal(err)
	}

	got, err := LoadCredentials("github.com", "octocat")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.ClientID != want.ClientID {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("expiry mismatch: got %s want %s", got.ExpiresAt, want.ExpiresAt)
	}

	if err := DeleteCredentials("github.com", "octocat"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredentials("github.com", "octocat"); err == nil {
		t.Error("expected error loading deleted credentials")
	}
}
