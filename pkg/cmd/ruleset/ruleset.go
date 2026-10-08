package ruleset

import (
	"github.com/MakeNowJust/heredoc"
	cmdCheck "github.com/cli/cli/v2/pkg/cmd/ruleset/check"
	cmdCreate "github.com/cli/cli/v2/pkg/cmd/ruleset/create"
	cmdDelete "github.com/cli/cli/v2/pkg/cmd/ruleset/delete"
	cmdList "github.com/cli/cli/v2/pkg/cmd/ruleset/list"
	cmdUpdate "github.com/cli/cli/v2/pkg/cmd/ruleset/update"
	cmdView "github.com/cli/cli/v2/pkg/cmd/ruleset/view"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdRuleset(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ruleset <command>",
		Short: "Manage repo rulesets",
		Long: heredoc.Doc(`
			Repository rulesets are a way to define a set of rules that apply to a repository.
			These commands allow you to view and manage them.
		`),
		Aliases: []string{"rs"},
		Example: heredoc.Doc(`
			$ gh ruleset list
			$ gh ruleset create protect-main --include-refs "~DEFAULT_BRANCH" --required-approvals 2
			$ gh ruleset view 42
			$ gh ruleset update 42 --enforcement disabled
			$ gh ruleset delete 42
			$ gh ruleset check branch-name
		`),
	}

	cmdutil.EnableRepoOverride(cmd, f)
	cmd.AddCommand(cmdList.NewCmdList(f, nil))
	cmd.AddCommand(cmdView.NewCmdView(f, nil))
	cmd.AddCommand(cmdCheck.NewCmdCheck(f, nil))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f, nil))
	cmd.AddCommand(cmdUpdate.NewCmdUpdate(f, nil))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f, nil))

	return cmd
}
