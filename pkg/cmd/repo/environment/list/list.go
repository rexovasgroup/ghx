package list

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/tableprinter"
	"github.com/cli/cli/v2/internal/text"
	"github.com/cli/cli/v2/pkg/cmd/repo/environment/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type listOptions struct {
	BaseRepo            func() (ghrepo.Interface, error)
	EnvironmentClient   EnvironmentListClient
	IO                  *iostreams.IOStreams

	Exporter cmdutil.Exporter
}

type EnvironmentListClient interface {
	List(repo ghrepo.Interface) ([]shared.Environment, int, error)
}

func NewCmdList(f *cmdutil.Factory, runF func(*listOptions) error) *cobra.Command {
	opts := &listOptions{
		IO: f.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments for a repository",
		Long: heredoc.Doc(`
			List deployment environments configured for a repository.

			Displays environment name, deployment branch policy, and
			the number of protection rules.
		`),
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo

			httpClient, err := f.HttpClient()
			if err != nil {
				return err
			}
			opts.EnvironmentClient = &EnvironmentLister{HTTPClient: httpClient}

			if runF != nil {
				return runF(opts)
			}

			return listRun(opts)
		},
	}

	cmdutil.AddJSONFlags(cmd, &opts.Exporter, shared.EnvironmentFields)

	return cmd
}

func listRun(opts *listOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	opts.IO.StartProgressIndicator()
	environments, totalCount, err := opts.EnvironmentClient.List(repo)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	cs := opts.IO.ColorScheme()

	if len(environments) == 0 {
		return cmdutil.NewNoResultsError(
			fmt.Sprintf("no environments found in %s", cs.Bold(ghrepo.FullName(repo))),
		)
	}

	if opts.Exporter != nil {
		return opts.Exporter.Write(opts.IO, environments)
	}

	if opts.IO.IsStdoutTTY() {
		title := fmt.Sprintf(
			"Showing %s in %s",
			text.Pluralize(totalCount, "environment"),
			cs.Bold(ghrepo.FullName(repo)),
		)
		fmt.Fprintf(opts.IO.Out, "\n%s\n\n", title)
	}

	tp := tableprinter.New(opts.IO, tableprinter.WithHeader("NAME", "BRANCH POLICY", "PROTECTION RULES"))

	for _, env := range environments {
		tp.AddField(env.Name)
		tp.AddField(branchPolicyLabel(env.DeploymentBranchPolicy))
		tp.AddField(strconv.Itoa(len(env.ProtectionRules)))
		tp.EndRow()
	}

	return tp.Render()
}

func branchPolicyLabel(policy *shared.DeploymentBranchPolicy) string {
	if policy == nil {
		return "all"
	}
	if policy.ProtectedBranches {
		return "protected"
	}
	if policy.CustomBranchPolicies {
		return "custom"
	}
	return "all"
}
