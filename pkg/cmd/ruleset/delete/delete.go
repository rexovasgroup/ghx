package delete

import (
	"fmt"
	"net/http"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/ruleset/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type iprompter interface {
	ConfirmDeletion(string) error
}

type DeleteOptions struct {
	BaseRepo   func() (ghrepo.Interface, error)
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	Prompter   iprompter

	ID           string
	Organization string
	Confirmed    bool
}

func NewCmdDelete(f *cmdutil.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := DeleteOptions{
		HttpClient: f.HttpClient,
		IO:         f.IOStreams,
		Prompter:   f.Prompter,
	}

	cmd := &cobra.Command{
		Use:   "delete <ruleset-id>",
		Short: "Delete a ruleset",
		Args:  cmdutil.ExactArgs(1, "cannot delete ruleset: ID argument required"),
		RunE: func(c *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo
			opts.ID = args[0]

			if !opts.IO.CanPrompt() && !opts.Confirmed {
				return cmdutil.FlagErrorf("--yes required when not running interactively")
			}

			if runF != nil {
				return runF(&opts)
			}
			return deleteRun(&opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Organization, "org", "o", "", "Organization that owns the ruleset")
	cmd.Flags().BoolVar(&opts.Confirmed, "yes", false, "Confirm deletion without prompting")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}

	var hostname, path, entityName string

	if opts.Organization != "" {
		hostname, err = ghHostForOrg(opts)
		if err != nil {
			return err
		}
		entityName = opts.Organization
		path = fmt.Sprintf("orgs/%s/rulesets/%s", opts.Organization, opts.ID)
	} else {
		baseRepo, err := opts.BaseRepo()
		if err != nil {
			return err
		}
		hostname = baseRepo.RepoHost()
		entityName = ghrepo.FullName(baseRepo)
		path = fmt.Sprintf("repos/%s/%s/rulesets/%s", baseRepo.RepoOwner(), baseRepo.RepoName(), opts.ID)
	}

	if !opts.Confirmed {
		// Fetch the ruleset to get its name for the confirmation prompt
		var rs *shared.RulesetREST
		if opts.Organization != "" {
			rs, err = shared.GetOrgRuleset(httpClient, opts.Organization, opts.ID, hostname)
		} else {
			baseRepo, _ := opts.BaseRepo()
			rs, err = shared.GetRepoRuleset(httpClient, baseRepo, opts.ID)
		}
		if err != nil {
			return err
		}
		if err := opts.Prompter.ConfirmDeletion(rs.Name); err != nil {
			return err
		}
	}

	opts.IO.StartProgressIndicator()
	apiClient := api.NewClientFromHTTP(httpClient)
	err = apiClient.REST(hostname, "DELETE", path, nil, nil)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "%s Ruleset %s deleted from %s\n", cs.SuccessIcon(), opts.ID, entityName)
	}

	return nil
}

func ghHostForOrg(opts *DeleteOptions) (string, error) {
	// When using --org, we need a hostname. Try the base repo if available, else default.
	if baseRepo, err := opts.BaseRepo(); err == nil {
		return baseRepo.RepoHost(), nil
	}
	return "github.com", nil
}
