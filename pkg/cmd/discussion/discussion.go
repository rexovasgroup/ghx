package discussion

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdDiscussion returns the top-level "discussion" command.
func NewCmdDiscussion(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discussion <command>",
		Short: "Manage discussions",
		Long:  "Work with GitHub Discussions.",
		Example: heredoc.Doc(`
			$ gh discussion list
			$ gh discussion create --category "General" --title "Hello"
			$ gh discussion view 123
		`),
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A discussion can be supplied as argument in any of the following formats:
				- by number, e.g. "123"; or
				- by URL, e.g. "https://github.com/OWNER/REPO/discussions/123".
			`),
		},
		GroupID: "core",
	}

	cmdutil.EnableRepoOverride(cmd, f)

	return cmd
}
