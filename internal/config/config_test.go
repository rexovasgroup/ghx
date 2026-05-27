package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cli/cli/v2/internal/gh"
	ghConfig "github.com/cli/go-gh/v2/pkg/config"
)

func newTestConfig() *cfg {
	return &cfg{
		cfg: ghConfig.ReadFromString(""),
	}
}

func TestNewConfigProvidesFallback(t *testing.T) {
	var spiedCfg *ghConfig.Config
	ghConfig.Read = func(fallback *ghConfig.Config) (*ghConfig.Config, error) {
		spiedCfg = fallback
		return fallback, nil
	}
	_, err := NewConfig()
	require.NoError(t, err)
	requireKeyWithValue(t, spiedCfg, []string{versionKey}, "1")
	requireKeyWithValue(t, spiedCfg, []string{gitProtocolKey}, "https")
	requireKeyWithValue(t, spiedCfg, []string{editorKey}, "")
	requireKeyWithValue(t, spiedCfg, []string{promptKey}, "enabled")
	requireKeyWithValue(t, spiedCfg, []string{pagerKey}, "")
	requireKeyWithValue(t, spiedCfg, []string{aliasesKey, "co"}, "pr checkout")
	requireKeyWithValue(t, spiedCfg, []string{httpUnixSocketKey}, "")
	requireKeyWithValue(t, spiedCfg, []string{browserKey}, "")
	requireKeyWithValue(t, spiedCfg, []string{colorLabelsKey}, "disabled")
}

func TestGetOrDefaultApplicationDefaults(t *testing.T) {
	tests := []struct {
		key             string
		expectedDefault string
	}{
		{gitProtocolKey, "https"},
		{editorKey, ""},
		{promptKey, "enabled"},
		{pagerKey, ""},
		{httpUnixSocketKey, ""},
		{browserKey, ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			// Given we have no top level configuration
			cfg := newTestConfig()

			// When we get a key that has no value, but has a default
			optionalEntry := cfg.GetOrDefault("", tt.key)

			// Then there is an entry with the default value, and source set as default
			entry := optionalEntry.Expect(fmt.Sprintf("expected there to be a value for %s", tt.key))
			require.Equal(t, tt.expectedDefault, entry.Value)
			require.Equal(t, gh.ConfigDefaultProvided, entry.Source)
		})
	}
}

func TestGetOrDefaultNonExistentKey(t *testing.T) {
	// Given we have no top level configuration
	cfg := newTestConfig()

	// When we get a key that has no value
	optionalEntry := cfg.GetOrDefault("", "non-existent-key")

	// Then it returns a None variant
	require.True(t, optionalEntry.IsNone(), "expected there to be no value")
}

func TestGetOrDefaultNonExistentHostSpecificKey(t *testing.T) {
	// Given have no top level configuration
	cfg := newTestConfig()

	// When we get a key for a host that has no value
	optionalEntry := cfg.GetOrDefault("non-existent-host", "non-existent-key")

	// Then it returns a None variant
	require.True(t, optionalEntry.IsNone(), "expected there to be no value")
}

func TestGetOrDefaultExistingTopLevelKey(t *testing.T) {
	// Given have a top level config entry
	cfg := newTestConfig()
	cfg.Set("", "top-level-key", "top-level-value")

	// When we get that key
	optionalEntry := cfg.GetOrDefault("non-existent-host", "top-level-key")

	// Then it returns a Some variant containing the correct value and a source of user
	entry := optionalEntry.Expect("expected there to be a value")
	require.Equal(t, "top-level-value", entry.Value)
	require.Equal(t, gh.ConfigUserProvided, entry.Source)
}

func TestGetOrDefaultExistingHostSpecificKey(t *testing.T) {
	// Given have a host specific config entry
	cfg := newTestConfig()
	cfg.Set("github.com", "host-specific-key", "host-specific-value")

	// When we get that key
	optionalEntry := cfg.GetOrDefault("github.com", "host-specific-key")

	// Then it returns a Some variant containing the correct value and a source of user
	entry := optionalEntry.Expect("expected there to be a value")
	require.Equal(t, "host-specific-value", entry.Value)
	require.Equal(t, gh.ConfigUserProvided, entry.Source)
}

func TestGetOrDefaultHostnameSpecificKeyFallsBackToTopLevel(t *testing.T) {
	// Given have a top level config entry
	cfg := newTestConfig()
	cfg.Set("", "key", "value")

	// When we get that key on a specific host
	optionalEntry := cfg.GetOrDefault("github.com", "key")

	// Then it returns a Some variant containing the correct value by falling back
	// to the top level config, with a source of user
	entry := optionalEntry.Expect("expected there to be a value")
	require.Equal(t, "value", entry.Value)
	require.Equal(t, gh.ConfigUserProvided, entry.Source)
}

func TestFallbackConfig(t *testing.T) {
	cfg := fallbackConfig()
	requireKeyWithValue(t, cfg, []string{gitProtocolKey}, "https")
	requireKeyWithValue(t, cfg, []string{editorKey}, "")
	requireKeyWithValue(t, cfg, []string{promptKey}, "enabled")
	requireKeyWithValue(t, cfg, []string{pagerKey}, "")
	requireKeyWithValue(t, cfg, []string{aliasesKey, "co"}, "pr checkout")
	requireKeyWithValue(t, cfg, []string{httpUnixSocketKey}, "")
	requireKeyWithValue(t, cfg, []string{browserKey}, "")
	requireKeyWithValue(t, cfg, []string{colorLabelsKey}, "disabled")
	requireNoKey(t, cfg, []string{"unknown"})
}

func TestSetTopLevelKey(t *testing.T) {
	c := newTestConfig()
	host := ""
	key := "top-level-key"
	val := "top-level-value"
	c.Set(host, key, val)
	requireKeyWithValue(t, c.cfg, []string{key}, val)
}

func TestSetHostSpecificKey(t *testing.T) {
	c := newTestConfig()
	host := "github.com"
	key := "host-level-key"
	val := "host-level-value"
	c.Set(host, key, val)
	requireKeyWithValue(t, c.cfg, []string{hostsKey, host, key}, val)
}

func TestSetUserSpecificKey(t *testing.T) {
	c := newTestConfig()
	host := "github.com"
	user := "test-user"
	c.cfg.Set([]string{hostsKey, host, userKey}, user)

	key := "host-level-key"
	val := "host-level-value"
	c.Set(host, key, val)
	requireKeyWithValue(t, c.cfg, []string{hostsKey, host, key}, val)
	requireKeyWithValue(t, c.cfg, []string{hostsKey, host, usersKey, user, key}, val)
}

func TestSetUserSpecificKeyNoUserPresent(t *testing.T) {
	c := newTestConfig()
	host := "github.com"
	key := "host-level-key"
	val := "host-level-value"
	c.Set(host, key, val)
	requireKeyWithValue(t, c.cfg, []string{hostsKey, host, key}, val)
	requireNoKey(t, c.cfg, []string{hostsKey, host, usersKey})
}

func TestActiveUserResolutionOrder(t *testing.T) {
	tests := []struct {
		name         string
		envUser      string
		gitAccount   string
		remoteOwner  string
		accountRules map[string]string // owner → account
		storedUser   string
		knownUsers   []string
		expectedUser string
		expectError  bool
	}{
		{
			name:         "GH_USER env var takes highest priority",
			envUser:      "env-user",
			gitAccount:   "git-user",
			storedUser:   "stored-user",
			knownUsers:   []string{"env-user", "git-user", "stored-user"},
			expectedUser: "env-user",
		},
		{
			name:         "GH_USER skipped if not authenticated",
			envUser:      "unknown-env-user",
			gitAccount:   "git-user",
			storedUser:   "stored-user",
			knownUsers:   []string{"git-user", "stored-user"},
			expectedUser: "git-user",
		},
		{
			name:         "git config github.account used when no env var",
			gitAccount:   "git-user",
			storedUser:   "stored-user",
			knownUsers:   []string{"git-user", "stored-user"},
			expectedUser: "git-user",
		},
		{
			name:         "git config skipped if not authenticated",
			gitAccount:   "unknown-git-user",
			storedUser:   "stored-user",
			knownUsers:   []string{"stored-user"},
			expectedUser: "stored-user",
		},
		{
			name:         "remote origin owner matched via account_rules",
			remoteOwner:  "MyWorkOrg",
			accountRules: map[string]string{"MyWorkOrg": "work-user"},
			storedUser:   "personal-user",
			knownUsers:   []string{"personal-user", "work-user"},
			expectedUser: "work-user",
		},
		{
			name:         "remote origin owner match is case-insensitive",
			remoteOwner:  "myworkorg",
			accountRules: map[string]string{"MyWorkOrg": "work-user"},
			storedUser:   "personal-user",
			knownUsers:   []string{"personal-user", "work-user"},
			expectedUser: "work-user",
		},
		{
			name:         "account_rules skipped if matched account not authenticated",
			remoteOwner:  "SomeOrg",
			accountRules: map[string]string{"SomeOrg": "unknown-user"},
			storedUser:   "stored-user",
			knownUsers:   []string{"stored-user"},
			expectedUser: "stored-user",
		},
		{
			name:         "no overrides falls back to stored user",
			storedUser:   "stored-user",
			knownUsers:   []string{"stored-user"},
			expectedUser: "stored-user",
		},
		{
			name:        "no overrides and no stored user returns error",
			knownUsers:  []string{},
			expectError: true,
		},
		{
			name:         "git config takes priority over account_rules",
			gitAccount:   "git-user",
			remoteOwner:  "SomeOrg",
			accountRules: map[string]string{"SomeOrg": "rule-user"},
			storedUser:   "stored-user",
			knownUsers:   []string{"git-user", "rule-user", "stored-user"},
			expectedUser: "git-user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override function pointers for testing
			origGit := gitConfigAccountFunc
			origRemote := remoteOriginOwnerFunc
			gitConfigAccountFunc = func() string { return tt.gitAccount }
			remoteOriginOwnerFunc = func() string { return tt.remoteOwner }
			t.Cleanup(func() {
				gitConfigAccountFunc = origGit
				remoteOriginOwnerFunc = origRemote
			})

			if tt.envUser != "" {
				t.Setenv("GH_USER", tt.envUser)
			}

			c := newTestConfig()
			host := "github.com"

			if tt.storedUser != "" {
				c.cfg.Set([]string{hostsKey, host, userKey}, tt.storedUser)
			}
			for _, user := range tt.knownUsers {
				c.cfg.Set([]string{hostsKey, host, usersKey, user, oauthTokenKey}, "test-token")
			}

			// Set up account_rules
			i := 0
			for owner, account := range tt.accountRules {
				c.cfg.Set([]string{accountRulesKey, fmt.Sprintf("%d", i), "owner"}, owner)
				c.cfg.Set([]string{accountRulesKey, fmt.Sprintf("%d", i), "account"}, account)
				i++
			}

			authCfg := c.Authentication().(*AuthConfig)
			user, err := authCfg.ActiveUser(host)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedUser, user)
			}
		})
	}
}

func TestOwnerFromRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"SSH with .git", "git@github.com:acme-corp/some-repo.git", "acme-corp"},
		{"SSH without .git", "git@github.com:acme-corp/some-repo", "acme-corp"},
		{"HTTPS with .git", "https://github.com/my-org/repo.git", "my-org"},
		{"HTTPS without .git", "https://github.com/my-org/repo", "my-org"},
		{"SSH with custom host alias", "git@github-work:acme-corp/repo.git", "acme-corp"},
		{"ssh:// protocol", "ssh://git@github.com/other-org/repo.git", "other-org"},
		{"empty string", "", ""},
		{"no path", "https://github.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ownerFromRemoteURL(tt.url)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTelemetry(t *testing.T) {
	t.Run("returns default when not configured", func(t *testing.T) {
		c := newTestConfig()

		entry := c.Telemetry()

		require.Equal(t, "enabled", entry.Value)
		require.Equal(t, gh.ConfigDefaultProvided, entry.Source)
	})

	t.Run("returns user configured value", func(t *testing.T) {
		c := newTestConfig()
		c.Set("", telemetryKey, "disabled")

		entry := c.Telemetry()

		require.Equal(t, "disabled", entry.Value)
		require.Equal(t, gh.ConfigUserProvided, entry.Source)
	})

	t.Run("returns log when configured", func(t *testing.T) {
		c := newTestConfig()
		c.Set("", telemetryKey, "log")

		entry := c.Telemetry()

		require.Equal(t, "log", entry.Value)
		require.Equal(t, gh.ConfigUserProvided, entry.Source)
	})
}
