package extension

import (
	"context"

	"github.com/cli/cli/v2/git"
)

type gitClient interface {
	CheckoutBranch(branch string) error
	Clone(cloneURL string, args []string) (string, error)
	CommandOutput(args []string) ([]byte, error)
	Config(name string) (string, error)
	Fetch(remote string, refspec string) error
	ForRepo(repoDir string) gitClient
	Pull(remote, branch string) error
	Remotes() (git.RemoteSet, error)
}

type gitExecuter struct {
	client *git.Client
}

// CheckoutBranch checks out the specified branch in the repository.
func (g *gitExecuter) CheckoutBranch(branch string) error {
	return g.client.CheckoutBranch(context.Background(), branch)
}

// Clone clones a repository from the given URL with optional arguments.
func (g *gitExecuter) Clone(cloneURL string, cloneArgs []string) (string, error) {
	return g.client.Clone(context.Background(), cloneURL, cloneArgs)
}

// CommandOutput runs a git command and returns its output.
func (g *gitExecuter) CommandOutput(args []string) ([]byte, error) {
	cmd, err := g.client.Command(context.Background(), args...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

// Config retrieves a git configuration value by name.
func (g *gitExecuter) Config(name string) (string, error) {
	return g.client.Config(context.Background(), name)
}

// Fetch fetches refs from the specified remote.
func (g *gitExecuter) Fetch(remote string, refspec string) error {
	return g.client.Fetch(context.Background(), remote, refspec)
}

// ForRepo returns a new gitClient scoped to the given repository directory.
func (g *gitExecuter) ForRepo(repoDir string) gitClient {
	gc := g.client.Copy()
	gc.RepoDir = repoDir
	return &gitExecuter{client: gc}
}

// Pull pulls changes from the specified remote and branch.
func (g *gitExecuter) Pull(remote, branch string) error {
	return g.client.Pull(context.Background(), remote, branch)
}

// Remotes returns the set of configured git remotes.
func (g *gitExecuter) Remotes() (git.RemoteSet, error) {
	return g.client.Remotes(context.Background())
}
