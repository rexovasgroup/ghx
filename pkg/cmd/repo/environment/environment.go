package environment

import (
	"github.com/MakeNowJust/heredoc"
	cmdCreate "github.com/cli/cli/v2/pkg/cmd/repo/environment/create"
	cmdDelete "github.com/cli/cli/v2/pkg/cmd/repo/environment/delete"
	cmdList "github.com/cli/cli/v2/pkg/cmd/repo/environment/list"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdEnvironment(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environment <command>",
		Short: "Manage repository environments",
		Long: heredoc.Doc(`
			Work with GitHub repository deployment environments.

			Environments are used to describe deployment targets like production,
			staging, or development, and can be configured with protection rules
			and secrets.
		`),
		Aliases: []string{"env"},
	}
	cmdutil.EnableRepoOverride(cmd, f)

	cmd.AddCommand(cmdList.NewCmdList(f, nil))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f, nil))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f, nil))

	return cmd
}
