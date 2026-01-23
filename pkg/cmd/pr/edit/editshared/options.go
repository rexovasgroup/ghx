// Package editshared contains shared types and utilities for the pr edit command variants
package editshared

import (
	"net/http"
	"strings"

	"github.com/cli/cli/v2/api"
	fd "github.com/cli/cli/v2/internal/featuredetection"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/pr/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

// EditOptions contains all the options for the pr edit command
type EditOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams

	Finder          shared.PRFinder
	Surveyor        Surveyor
	Fetcher         EditableOptionsFetcher
	EditorRetriever EditorRetriever
	Prompter        shared.EditPrompter
	Detector        fd.Detector
	BaseRepo        func() (ghrepo.Interface, error)

	SelectorArg string
	Interactive bool

	// Flag-related fields that need to be accessed during RunE
	BodyFile        string
	RemoveMilestone bool

	shared.Editable
}

// Surveyor handles interactive field editing
type Surveyor interface {
	FieldsToEdit(*shared.Editable) error
	EditFields(*shared.Editable, string) error
}

// EditableOptionsFetcher fetches options for editable fields
type EditableOptionsFetcher interface {
	EditableOptionsFetch(*api.Client, ghrepo.Interface, *shared.Editable, gh.ProjectsV1Support) error
}

// EditorRetriever retrieves the configured editor
type EditorRetriever interface {
	Retrieve() (string, error)
}

// DefaultSurveyor is the default implementation of Surveyor
type DefaultSurveyor struct {
	P shared.EditPrompter
}

func (s DefaultSurveyor) FieldsToEdit(editable *shared.Editable) error {
	return shared.FieldsToEditSurvey(s.P, editable)
}

func (s DefaultSurveyor) EditFields(editable *shared.Editable, editorCmd string) error {
	return shared.EditFieldsSurvey(s.P, editable, editorCmd)
}

// DefaultFetcher is the default implementation of EditableOptionsFetcher
type DefaultFetcher struct{}

func (f DefaultFetcher) EditableOptionsFetch(client *api.Client, repo ghrepo.Interface, opts *shared.Editable, projectsV1Support gh.ProjectsV1Support) error {
	return shared.FetchOptions(client, repo, opts, projectsV1Support)
}

// DefaultEditorRetriever is the default implementation of EditorRetriever
type DefaultEditorRetriever struct {
	Config func() (gh.Config, error)
}

func (e DefaultEditorRetriever) Retrieve() (string, error) {
	return cmdutil.DetermineEditor(e.Config)
}

// ParseFlags processes the command flags and populates the EditOptions accordingly.
// This should be called at the start of each variant's RunE.
func (opts *EditOptions) ParseFlags(cmd *cobra.Command, args []string, f *cmdutil.Factory) error {
	opts.Finder = shared.NewFinder(f)
	opts.BaseRepo = f.BaseRepo

	if len(args) > 0 {
		opts.SelectorArg = args[0]
	}

	if opts.SelectorArg != "" {
		// If a URL is provided, we need to parse it to override the
		// base repository, especially the hostname part.
		if baseRepo, _, _, err := shared.ParseURL(opts.SelectorArg); err == nil {
			opts.BaseRepo = func() (ghrepo.Interface, error) {
				return baseRepo, nil
			}
		}
	}

	flags := cmd.Flags()

	bodyProvided := flags.Changed("body")
	bodyFileProvided := opts.BodyFile != ""

	if err := cmdutil.MutuallyExclusive(
		"specify only one of `--body` or `--body-file`",
		bodyProvided,
		bodyFileProvided,
	); err != nil {
		return err
	}
	if bodyProvided || bodyFileProvided {
		opts.Editable.Body.Edited = true
		if bodyFileProvided {
			b, err := cmdutil.ReadFile(opts.BodyFile, opts.IO.In)
			if err != nil {
				return err
			}
			opts.Editable.Body.Value = string(b)
		}
	}

	if err := cmdutil.MutuallyExclusive(
		"specify only one of `--milestone` or `--remove-milestone`",
		flags.Changed("milestone"),
		opts.RemoveMilestone,
	); err != nil {
		return err
	}

	if flags.Changed("title") {
		opts.Editable.Title.Edited = true
	}
	if flags.Changed("body") {
		opts.Editable.Body.Edited = true
	}
	if flags.Changed("base") {
		opts.Editable.Base.Edited = true
	}
	if flags.Changed("add-reviewer") || flags.Changed("remove-reviewer") {
		opts.Editable.Reviewers.Edited = true
	}
	if flags.Changed("add-assignee") || flags.Changed("remove-assignee") {
		opts.Editable.Assignees.Edited = true
	}
	if flags.Changed("add-label") || flags.Changed("remove-label") {
		opts.Editable.Labels.Edited = true
	}
	if flags.Changed("add-project") || flags.Changed("remove-project") {
		opts.Editable.Projects.Edited = true
	}
	if flags.Changed("milestone") || opts.RemoveMilestone {
		opts.Editable.Milestone.Edited = true
	}

	if !opts.Editable.Dirty() {
		opts.Interactive = true
	}

	if opts.Interactive && !opts.IO.CanPrompt() {
		return cmdutil.FlagErrorf("--title, --body, --reviewer, --assignee, --label, --project, or --milestone required when not running interactively")
	}

	return nil
}

// PartitionUsersAndTeams splits reviewer identifiers into user logins and team slugs.
// Team identifiers are in the form "org/slug"; only the slug portion is returned for teams.
func PartitionUsersAndTeams(values []string) (users []string, teams []string) {
	for _, v := range values {
		if strings.ContainsRune(v, '/') {
			parts := strings.SplitN(v, "/", 2)
			if len(parts) == 2 && parts[1] != "" {
				teams = append(teams, parts[1])
			}
		} else if v != "" {
			users = append(users, v)
		}
	}
	return
}
