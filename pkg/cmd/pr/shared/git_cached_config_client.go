package shared

import (
	"context"

	"github.com/cli/cli/v2/git"
)

var _ GitConfigClient = &CachedBranchConfigGitConfigClient{}

// CachedBranchConfigGitConfigClient wraps a GitConfigClient to return a pre-fetched BranchConfig instead of querying git.
type CachedBranchConfigGitConfigClient struct {
	CachedBranchConfig git.BranchConfig
	GitConfigClient
}

// ReadBranchConfig returns the cached BranchConfig, ignoring the branch name argument.
func (c CachedBranchConfigGitConfigClient) ReadBranchConfig(ctx context.Context, branchName string) (git.BranchConfig, error) {
	return c.CachedBranchConfig, nil
}
