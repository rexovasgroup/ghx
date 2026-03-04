package extension

import (
	"github.com/cli/cli/v2/git"
	"github.com/stretchr/testify/mock"
)

type mockGitClient struct {
	mock.Mock
}

// CheckoutBranch mocks checking out a branch.
func (g *mockGitClient) CheckoutBranch(branch string) error {
	args := g.Called(branch)
	return args.Error(0)
}

// Clone mocks cloning a repository.
func (g *mockGitClient) Clone(cloneURL string, cloneArgs []string) (string, error) {
	args := g.Called(cloneURL, cloneArgs)
	return args.String(0), args.Error(1)
}

// CommandOutput mocks running a git command and returning its output.
func (g *mockGitClient) CommandOutput(commandArgs []string) ([]byte, error) {
	args := g.Called(commandArgs)
	return []byte(args.String(0)), args.Error(1)
}

// Config mocks retrieving a git configuration value.
func (g *mockGitClient) Config(name string) (string, error) {
	args := g.Called(name)
	return args.String(0), args.Error(1)
}

// Fetch mocks fetching refs from a remote.
func (g *mockGitClient) Fetch(remote string, refspec string) error {
	args := g.Called(remote, refspec)
	return args.Error(0)
}

// ForRepo mocks returning a gitClient scoped to a repository directory.
func (g *mockGitClient) ForRepo(repoDir string) gitClient {
	args := g.Called(repoDir)
	if v, ok := args.Get(0).(*mockGitClient); ok {
		return v
	}
	return nil
}

// Pull mocks pulling changes from a remote branch.
func (g *mockGitClient) Pull(remote, branch string) error {
	args := g.Called(remote, branch)
	return args.Error(0)
}

// Remotes mocks returning the set of configured git remotes.
func (g *mockGitClient) Remotes() (git.RemoteSet, error) {
	args := g.Called()
	return nil, args.Error(1)
}
