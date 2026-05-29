package delete

import (
	"fmt"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type deleteOptions struct {
	BaseRepo          func() (ghrepo.Interface, error)
	EnvironmentClient EnvironmentDeleteClient
	IO                *iostreams.IOStreams
	Prompter          prompter.Prompter

	Name      string
	Confirmed bool
}

type EnvironmentDeleteClient interface {
	Delete(repo ghrepo.Interface, name string) error
}

func NewCmdDelete(f *cmdutil.Factory, runF func(*deleteOptions) error) *cobra.Command {
	opts := &deleteOptions{
		IO:       f.IOStreams,
		Prompter: f.Prompter,
	}

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an environment",
		Long:  "Delete a deployment environment from a repository.",
		Args:  cmdutil.ExactArgs(1, "cannot delete environment: name argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo
			opts.Name = args[0]

			httpClient, err := f.HttpClient()
			if err != nil {
				return err
			}
			opts.EnvironmentClient = &EnvironmentDeleter{HTTPClient: httpClient}

			if !opts.IO.CanPrompt() && !opts.Confirmed {
				return cmdutil.FlagErrorf("--yes required when not running interactively")
			}

			if runF != nil {
				return runF(opts)
			}

			return deleteRun(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Confirmed, "yes", false, "Confirm deletion without prompting")

	return cmd
}

func deleteRun(opts *deleteOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	cs := opts.IO.ColorScheme()

	if opts.IO.CanPrompt() && !opts.Confirmed {
		err := opts.Prompter.ConfirmDeletion(opts.Name)
		if err != nil {
			return err
		}
	}

	opts.IO.StartProgressIndicator()
	err = opts.EnvironmentClient.Delete(repo, opts.Name)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		fmt.Fprintf(opts.IO.Out, "%s Environment %s deleted from %s\n",
			cs.SuccessIcon(),
			cs.Bold(opts.Name),
			cs.Bold(ghrepo.FullName(repo)),
		)
	}

	return nil
}
