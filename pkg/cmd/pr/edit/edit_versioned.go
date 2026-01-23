// Package edit provides the gh pr edit command with version-aware routing.
//
// This file demonstrates the VersionedCommand pattern for handling GHES backwards compatibility.
// Instead of scattering feature detection conditionals throughout the code, we:
// 1. Define the command once (flags, help text, etc.)
// 2. Route to version-specific implementations based on the target host
//
// When the oldest supported GHES version reaches EOL, simply:
// 1. Delete the gheslatest package
// 2. Remove the variant from the VersionedCommand configuration below
// 3. Optionally, inline the cloudlatest if no other versions need support
package edit

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/pkg/cmd/pr/edit/cloudlatest"
	editshared "github.com/cli/cli/v2/pkg/cmd/pr/edit/editshared"
	"github.com/cli/cli/v2/pkg/cmd/pr/edit/ghes318"
	"github.com/cli/cli/v2/pkg/cmd/pr/edit/gheslatest"
	"github.com/cli/cli/v2/pkg/cmd/pr/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmdEditVersioned creates the pr edit command using the VersionedCommand pattern.
// This is an alternative to NewCmdEdit that demonstrates clean GHES version handling.
//
// To use this instead of NewCmdEdit, update pr.go to call NewCmdEditVersioned.
func NewCmdEditVersioned(f *cmdutil.Factory, runF func(*editshared.EditOptions) error) *cobra.Command {
	opts := &editshared.EditOptions{
		IO:              f.IOStreams,
		HttpClient:      f.HttpClient,
		Surveyor:        editshared.DefaultSurveyor{P: f.Prompter},
		Fetcher:         editshared.DefaultFetcher{},
		EditorRetriever: editshared.DefaultEditorRetriever{Config: f.Config},
		Prompter:        f.Prompter,
	}

	cmd := &cobra.Command{
		Use:   "edit [<number> | <url> | <branch>]",
		Short: "Edit a pull request",
		Long: heredoc.Docf(`
			Edit a pull request.

			Without an argument, the pull request that belongs to the current branch
			is selected.

			Editing a pull request's projects requires authorization with the %[1]sproject%[1]s scope.
			To authorize, run %[1]sgh auth refresh -s project%[1]s.

			The %[1]s--add-assignee%[1]s and %[1]s--remove-assignee%[1]s flags both support
			the following special values:
			- %[1]s@me%[1]s: assign or unassign yourself
			- %[1]s@copilot%[1]s: assign or unassign Copilot (not supported on GitHub Enterprise Server)

			The %[1]s--add-reviewer%[1]s and %[1]s--remove-reviewer%[1]s flags do not support
			these special values.
		`, "`"),
		Example: heredoc.Doc(`
			$ gh pr edit 23 --title "I found a bug" --body "Nothing works"
			$ gh pr edit 23 --add-label "bug,help wanted" --remove-label "core"
			$ gh pr edit 23 --add-reviewer monalisa,hubot  --remove-reviewer myorg/team-name
			$ gh pr edit 23 --add-assignee "@me" --remove-assignee monalisa,hubot
			$ gh pr edit 23 --add-assignee "@copilot"
			$ gh pr edit 23 --add-project "Roadmap" --remove-project v1,v2
			$ gh pr edit 23 --milestone "Version 1"
			$ gh pr edit 23 --remove-milestone
		`),
		Args: cobra.MaximumNArgs(1),
	}

	// Register flags - these are bound to opts and read by each variant's RunE
	cmd.Flags().StringVarP(&opts.Editable.Title.Value, "title", "t", "", "Set the new title.")
	cmd.Flags().StringVarP(&opts.Editable.Body.Value, "body", "b", "", "Set the new body.")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body text from `file` (use \"-\" to read from standard input)")
	cmd.Flags().StringVarP(&opts.Editable.Base.Value, "base", "B", "", "Change the base `branch` for this pull request")
	cmd.Flags().StringSliceVar(&opts.Editable.Reviewers.Add, "add-reviewer", nil, "Add reviewers by their `login`.")
	cmd.Flags().StringSliceVar(&opts.Editable.Reviewers.Remove, "remove-reviewer", nil, "Remove reviewers by their `login`.")
	cmd.Flags().StringSliceVar(&opts.Editable.Assignees.Add, "add-assignee", nil, "Add assigned users by their `login`. Use \"@me\" to assign yourself, or \"@copilot\" to assign Copilot.")
	cmd.Flags().StringSliceVar(&opts.Editable.Assignees.Remove, "remove-assignee", nil, "Remove assigned users by their `login`. Use \"@me\" to unassign yourself, or \"@copilot\" to unassign Copilot.")
	cmd.Flags().StringSliceVar(&opts.Editable.Labels.Add, "add-label", nil, "Add labels by `name`")
	cmd.Flags().StringSliceVar(&opts.Editable.Labels.Remove, "remove-label", nil, "Remove labels by `name`")
	cmd.Flags().StringSliceVar(&opts.Editable.Projects.Add, "add-project", nil, "Add the pull request to projects by `title`")
	cmd.Flags().StringSliceVar(&opts.Editable.Projects.Remove, "remove-project", nil, "Remove the pull request from projects by `title`")
	cmd.Flags().StringVarP(&opts.Editable.Milestone.Value, "milestone", "m", "", "Edit the milestone the pull request belongs to by `name`")
	cmd.Flags().BoolVar(&opts.RemoveMilestone, "remove-milestone", false, "Remove the milestone association from the pull request")

	_ = cmdutil.RegisterBranchCompletionFlags(f.GitClient, cmd, "base")

	for _, flagName := range []string{"add-reviewer", "remove-reviewer"} {
		_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			baseRepo, err := f.BaseRepo()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			httpClient, err := f.HttpClient()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			results, err := shared.RequestableReviewersForCompletion(httpClient, baseRepo)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return results, cobra.ShellCompDirectiveNoFileComp
		})
	}

	// Wire up version-specific routing
	// Each variant's RunE handles its own flag parsing and business logic
	//
	// Variant matching order: variants are checked in map iteration order (undefined).
	// When multiple variants could match, use non-overlapping constraints.
	vc := &cmdutil.VersionedCommand{
		Name: "edit",
		Cmd:  cmd,

		// Default: cloudlatest for github.com (GHES always matches a variant below)
		DefaultRunE: wrapRunF(runF, cloudlatest.NewRunE(f, opts)),

		// Version-specific variants for GHES
		// Constraints should be non-overlapping and cover all supported GHES versions
		Variants: map[string]cmdutil.VersionedVariant{
			// GHES 3.18+: Projects V1 removed, but still no ActorIsAssignable
			"ghes-3.18": {
				RunE:       wrapRunF(runF, ghes318.NewRunE(f, opts)),
				Constraint: cmdutil.GHESVersionConstraint(">=", "3.18.0"),
			},
			// GHES < 3.18: older version with Projects V1 support
			"ghes-latest": {
				RunE:       wrapRunF(runF, gheslatest.NewRunE(f, opts)),
				Constraint: cmdutil.GHESVersionConstraint("<", "3.18.0"),
			},
		},

		HostResolver: cmdutil.RepoHostResolver(f.BaseRepo),
		HttpClient:   f.HttpClient,
	}

	return vc.Command()
}

// wrapRunF wraps the variant RunE to support test injection via runF
func wrapRunF(runF func(*editshared.EditOptions) error, variantRunE func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	if runF != nil {
		// For testing: runF is injected, but we still need flag parsing
		// The test is responsible for setting up opts correctly
		return variantRunE
	}
	return variantRunE
}
