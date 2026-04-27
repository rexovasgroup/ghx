package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiscussionArg(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		wantNum   int
		wantOwner string
		wantRepo  string
		wantHost  string
		wantErr   string
	}{
		{
			name:    "empty",
			arg:     "",
			wantErr: `invalid discussion argument: ""`,
		},
		{
			name:    "whitespaces",
			arg:     "  ",
			wantErr: `invalid discussion argument: "  "`,
		},
		{
			name:    "invalid string",
			arg:     "not-a-number",
			wantErr: `invalid discussion argument: "not-a-number"`,
		},
		{
			name:    "hash only",
			arg:     "#",
			wantErr: `invalid discussion argument: "#"`,
		},
		{
			name:    "hash non-numeric",
			arg:     "#abc",
			wantErr: `invalid discussion argument: "#abc"`,
		},
		{
			name:    "URL with wrong path",
			arg:     "https://github.com/owner/repo/issues/10",
			wantErr: `invalid discussion URL: "https://github.com/owner/repo/issues/10"`,
		},
		{
			name:    "URL missing number",
			arg:     "https://github.com/owner/repo/discussions/",
			wantErr: `invalid discussion URL: "https://github.com/owner/repo/discussions/"`,
		},
		{
			name:    "zero",
			arg:     "0",
			wantNum: 0,
		},
		{
			name:    "plain number",
			arg:     "42",
			wantNum: 42,
		},
		{
			name:    "hash number",
			arg:     "#99",
			wantNum: 99,
		},
		{
			name:      "HTTPS URL",
			arg:       "https://github.com/cli/cli/discussions/123",
			wantNum:   123,
			wantOwner: "cli",
			wantRepo:  "cli",
			wantHost:  "github.com",
		},
		{
			name:      "HTTP URL",
			arg:       "http://github.com/owner/repo/discussions/7",
			wantNum:   7,
			wantOwner: "owner",
			wantRepo:  "repo",
			wantHost:  "github.com",
		},
		{
			name:      "GHES URL",
			arg:       "https://git.example.com/org/project/discussions/55",
			wantNum:   55,
			wantOwner: "org",
			wantRepo:  "project",
			wantHost:  "git.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, repo, err := ParseDiscussionArg(tt.arg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantNum, num)

			if tt.wantOwner != "" || tt.wantRepo != "" || tt.wantHost != "" {
				require.NotNil(t, repo)
				assert.Equal(t, tt.wantOwner, repo.RepoOwner())
				assert.Equal(t, tt.wantRepo, repo.RepoName())
				assert.Equal(t, tt.wantHost, repo.RepoHost())
			} else {
				assert.Nil(t, repo)
			}
		})
	}
}
