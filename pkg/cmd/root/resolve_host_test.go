package root

import (
	"testing"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestResolveTargetHost(t *testing.T) {
	tests := []struct {
		name        string
		repoFlag    string
		hostnameFlag bool
		hostname    string
		ghRepoEnv   string
		baseRepo    ghrepo.Interface
		baseRepoErr error
		defaultHost string
		wantHost    string
	}{
		{
			name:        "repo flag with host takes priority",
			repoFlag:    "ghes.example.com/org/repo",
			defaultHost: "github.com",
			wantHost:    "ghes.example.com",
		},
		{
			name:        "repo flag without host uses default",
			repoFlag:    "org/repo",
			defaultHost: "github.com",
			wantHost:    "github.com",
		},
		{
			name:        "GH_REPO env with host",
			ghRepoEnv:   "ghes.example.com/org/repo",
			defaultHost: "github.com",
			wantHost:    "ghes.example.com",
		},
		{
			name:         "hostname flag takes priority over BaseRepo",
			hostnameFlag: true,
			hostname:     "ghes.example.com",
			baseRepo:     ghrepo.NewWithHost("org", "repo", "github.com"),
			defaultHost:  "github.com",
			wantHost:     "ghes.example.com",
		},
		{
			name:        "BaseRepo host from git remote",
			baseRepo:    ghrepo.NewWithHost("org", "repo", "ghes.example.com"),
			defaultHost: "github.com",
			wantHost:    "ghes.example.com",
		},
		{
			name:        "falls back to default host",
			baseRepoErr: assert.AnError,
			defaultHost: "github.com",
			wantHost:    "github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ghRepoEnv != "" {
				t.Setenv("GH_REPO", tt.ghRepoEnv)
			}

			cmd := &cobra.Command{Use: "test"}
			if tt.repoFlag != "" || tt.ghRepoEnv != "" {
				cmd.Flags().StringP("repo", "R", "", "")
				if tt.repoFlag != "" {
					cmd.Flags().Set("repo", tt.repoFlag)
				}
			}
			if tt.hostnameFlag {
				cmd.Flags().StringP("hostname", "h", "", "")
				if tt.hostname != "" {
					cmd.Flags().Set("hostname", tt.hostname)
				}
			}

			f := &cmdutil.Factory{
				BaseRepo: func() (ghrepo.Interface, error) {
					if tt.baseRepoErr != nil {
						return nil, tt.baseRepoErr
					}
					if tt.baseRepo != nil {
						return tt.baseRepo, nil
					}
					return nil, assert.AnError
				},
			}

			got := resolveTargetHost(cmd, f, tt.defaultHost)
			assert.Equal(t, tt.wantHost, got)
		})
	}
}
