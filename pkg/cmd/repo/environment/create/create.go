package create

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/repo/environment/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type CreateOptions struct {
	BaseRepo          func() (ghrepo.Interface, error)
	EnvironmentClient EnvironmentCreateClient
	HttpClient        func() (*http.Client, error)
	IO                *iostreams.IOStreams

	Name                   string
	WaitTimer              int
	Reviewers              []string
	DeploymentBranchPolicy string
	PreventSelfReview      bool
	FromJSON               string
}

type EnvironmentCreateClient interface {
	CreateOrUpdate(repo ghrepo.Interface, name string, request EnvironmentCreateRequest) (*shared.Environment, error)
	CreateOrUpdateRaw(repo ghrepo.Interface, name string, body []byte) (*shared.Environment, error)
}

func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		HttpClient: f.HttpClient,
		IO:         f.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create or update an environment",
		Long: heredoc.Doc(`
			Create or update a deployment environment for a repository.

			If the environment already exists, it will be updated with the
			provided settings.
		`),
		Example: heredoc.Doc(`
			# Create a simple environment
			$ gh repo environment create staging

			# Create with a wait timer and reviewers
			$ gh repo environment create production --wait-timer 30 --reviewer monalisa --reviewer my-org/my-team

			# Create with deployment branch policy
			$ gh repo environment create production --deployment-branch-policy protected

			# Create from a JSON file
			$ gh repo environment create production --from-json env.json
		`),
		Args: cmdutil.ExactArgs(1, "cannot create environment: name argument required"),
		RunE: func(c *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo
			opts.Name = args[0]

			httpClient, err := f.HttpClient()
			if err != nil {
				return err
			}
			opts.EnvironmentClient = &EnvironmentCreator{HTTPClient: httpClient}

			if opts.DeploymentBranchPolicy != "" && opts.DeploymentBranchPolicy != "protected" && opts.DeploymentBranchPolicy != "custom" {
				return cmdutil.FlagErrorf("--deployment-branch-policy must be %q or %q", "protected", "custom")
			}

			if runF != nil {
				return runF(opts)
			}

			return createRun(opts)
		},
	}

	cmd.Flags().IntVar(&opts.WaitTimer, "wait-timer", 0, "Minutes to wait before allowing deployments (0-43200)")
	cmd.Flags().StringSliceVar(&opts.Reviewers, "reviewer", nil, "Required reviewer (user login or org/team slug); can be specified multiple times")
	cmd.Flags().StringVar(&opts.DeploymentBranchPolicy, "deployment-branch-policy", "", "Deployment branch policy: \"protected\" or \"custom\"")
	cmd.Flags().BoolVar(&opts.PreventSelfReview, "prevent-self-review", false, "Prevent users from approving their own deployments")
	cmd.Flags().StringVar(&opts.FromJSON, "from-json", "", "Path to JSON file with environment configuration (use - for stdin)")

	return cmd
}

func createRun(opts *CreateOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	if opts.FromJSON != "" {
		return createFromJSON(opts, repo)
	}

	request := EnvironmentCreateRequest{}

	if opts.WaitTimer > 0 {
		request.WaitTimer = &opts.WaitTimer
	}

	if opts.PreventSelfReview {
		request.PreventSelfReview = &opts.PreventSelfReview
	}

	if opts.DeploymentBranchPolicy != "" {
		switch opts.DeploymentBranchPolicy {
		case "protected":
			request.DeploymentBranchPolicy = &shared.DeploymentBranchPolicy{
				ProtectedBranches:    true,
				CustomBranchPolicies: false,
			}
		case "custom":
			request.DeploymentBranchPolicy = &shared.DeploymentBranchPolicy{
				ProtectedBranches:    false,
				CustomBranchPolicies: true,
			}
		}
	}

	if len(opts.Reviewers) > 0 {
		httpClient, err := opts.HttpClient()
		if err != nil {
			return err
		}
		reviewers, err := resolveReviewers(httpClient, repo, opts.Reviewers)
		if err != nil {
			return err
		}
		request.Reviewers = reviewers
	}

	opts.IO.StartProgressIndicator()
	env, err := opts.EnvironmentClient.CreateOrUpdate(repo, opts.Name, request)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "%s Environment %s configured on %s\n",
			cs.SuccessIconWithColor(cs.Green),
			cs.Bold(env.Name),
			cs.Bold(ghrepo.FullName(repo)),
		)
	}

	return nil
}

func createFromJSON(opts *CreateOptions, repo ghrepo.Interface) error {
	var data []byte
	var err error

	if opts.FromJSON == "-" {
		data, err = io.ReadAll(opts.IO.In)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(opts.FromJSON)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", opts.FromJSON, err)
		}
	}

	opts.IO.StartProgressIndicator()
	env, err := opts.EnvironmentClient.CreateOrUpdateRaw(repo, opts.Name, data)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "%s Environment %s configured on %s\n",
			cs.SuccessIconWithColor(cs.Green),
			cs.Bold(env.Name),
			cs.Bold(ghrepo.FullName(repo)),
		)
	}

	return nil
}

type userResponse struct {
	ID int `json:"id"`
}

type teamResponse struct {
	ID int `json:"id"`
}

// resolveReviewers converts user logins and org/team slugs to reviewer requests with IDs.
// Format: "username" for users, "org/team-slug" for teams.
func resolveReviewers(httpClient *http.Client, repo ghrepo.Interface, reviewers []string) ([]ReviewerRequest, error) {
	var result []ReviewerRequest
	hostname := repo.RepoHost()

	for _, r := range reviewers {
		// Check if it's a team (contains /)
		if isTeamSlug(r) {
			orgName, teamSlug := splitTeamSlug(r)
			id, err := resolveTeam(httpClient, hostname, orgName, teamSlug)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve team %q: %w", r, err)
			}
			result = append(result, ReviewerRequest{Type: "Team", ID: id})
		} else {
			id, err := resolveUser(httpClient, hostname, r)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve user %q: %w", r, err)
			}
			result = append(result, ReviewerRequest{Type: "User", ID: id})
		}
	}

	return result, nil
}

func isTeamSlug(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}

func splitTeamSlug(s string) (string, string) {
	for i, c := range s {
		if c == '/' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func resolveUser(httpClient *http.Client, hostname, login string) (int, error) {
	url := ghinstance.RESTPrefix(hostname) + fmt.Sprintf("users/%s", login)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return 0, api.HandleHTTPError(resp)
	}

	var user userResponse
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return 0, err
	}

	return user.ID, nil
}

func resolveTeam(httpClient *http.Client, hostname, org, slug string) (int, error) {
	url := ghinstance.RESTPrefix(hostname) + fmt.Sprintf("orgs/%s/teams/%s", org, slug)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return 0, api.HandleHTTPError(resp)
	}

	var team teamResponse
	err = json.NewDecoder(resp.Body).Decode(&team)
	if err != nil {
		return 0, err
	}

	return team.ID, nil
}
