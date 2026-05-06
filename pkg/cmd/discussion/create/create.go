package create

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/pkg/cmd/discussion/client"
	"github.com/cli/cli/v2/pkg/cmd/discussion/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

// CreateOptions holds the configuration for the discussion create command.
type CreateOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (ghrepo.Interface, error)
	Client     func() (client.DiscussionClient, error)
	Prompter   prompter.Prompter

	Title    string
	Body     string
	Category string
	Labels   []string
}

// NewCmdCreate returns a cobra command for creating a GitHub Discussion.
func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Prompter:   f.Prompter,
		Client:     shared.DiscussionClientFunc(f),
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new discussion",
		Long: heredoc.Doc(`
			Create a new GitHub Discussion in a repository.

			With '--title' and '--category', a discussion is created non-interactively.
			Omitting either flag triggers interactive prompts when connected to a terminal.

			The '--body' flag provides the discussion body. Without it you will be
			prompted to enter one in your default editor.
		`),
		Example: heredoc.Doc(`
			# Create interactively
			$ gh discussion create

			# Create non-interactively
			$ gh discussion create --title "My question" --category "Q&A" --body "Details here"
		`),
		Args: cmdutil.NoArgsQuoteReminder,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo
			if runF != nil {
				return runF(opts)
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Title for the discussion")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Body for the discussion")
	cmd.Flags().StringVarP(&opts.Category, "category", "c", "", "Category name or slug for the discussion")
	cmd.Flags().StringSliceVarP(&opts.Labels, "label", "l", nil, "Labels to apply to the discussion")

	return cmd
}

func createRun(opts *CreateOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	c, err := opts.Client()
	if err != nil {
		return err
	}

	categories, err := c.ListCategories(repo)
	if err != nil {
		return fmt.Errorf("fetching categories: %w", err)
	}

	interactive := opts.IO.CanPrompt()

	if opts.Title == "" {
		if !interactive {
			return cmdutil.FlagErrorf("--title required when not running interactively")
		}
		opts.Title, err = opts.Prompter.Input("Discussion title", "")
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.Title) == "" {
		return cmdutil.FlagErrorf("title cannot be blank")
	}

	var category *client.DiscussionCategory
	if opts.Category != "" {
		category, err = shared.MatchCategory(opts.Category, categories)
		if err != nil {
			return err
		}
	} else {
		if !interactive {
			return cmdutil.FlagErrorf("--category required when not running interactively")
		}
		names := make([]string, len(categories))
		for i, cat := range categories {
			names[i] = cat.Name
		}
		idx, err := opts.Prompter.Select("Discussion category", "", names)
		if err != nil {
			return err
		}
		category = &categories[idx]
	}

	if opts.Body == "" {
		if !interactive {
			return cmdutil.FlagErrorf("--body required when not running interactively")
		}
		opts.Body, err = opts.Prompter.MarkdownEditor("Discussion body", "", true)
		if err != nil {
			return err
		}
	}

	input := client.CreateDiscussionInput{
		CategoryID: category.ID,
		Title:      opts.Title,
		Body:       opts.Body,
		Labels:     opts.Labels,
	}

	discussion, err := c.Create(repo, input)
	if err != nil {
		return fmt.Errorf("creating discussion: %w", err)
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "%s Created discussion #%d: %s\n",
			cs.SuccessIcon(), discussion.Number, discussion.URL)
	} else {
		fmt.Fprintln(opts.IO.Out, discussion.URL)
	}

	return nil
}
