package list

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/browser"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/tableprinter"
	"github.com/cli/cli/v2/internal/text"
	"github.com/cli/cli/v2/pkg/cmd/discussion/client"
	"github.com/cli/cli/v2/pkg/cmd/discussion/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

// ListOptions holds the configuration for the discussion list command.
type ListOptions struct {
	IO       *iostreams.IOStreams
	BaseRepo func() (ghrepo.Interface, error)
	Browser  browser.Browser
	Client   func() (client.DiscussionClient, error)

	Author   string
	Category string
	Labels   []string
	State    string
	Limit    int
	Answered *bool
	Order    string

	WebMode  bool
	Exporter cmdutil.Exporter
	Now      func() time.Time
}

// NewCmdList creates the "discussion list" command.
func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:      f.IOStreams,
		Browser: f.Browser,
		Now:     time.Now,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discussions in a repository",
		Long: heredoc.Doc(`
			List discussions in a GitHub repository. By default, only open discussions
			are shown.
		`),
		Example: heredoc.Doc(`
			# List open discussions
			$ gh discussion list

			# List discussions with a specific category
			$ gh discussion list --category "General"

			# List closed discussions by author
			$ gh discussion list --state closed --author monalisa

			# List answered discussions as JSON
			$ gh discussion list --answered --json number,title,url
		`),
		Aliases: []string{"ls"},
		Args:    cmdutil.NoArgsQuoteReminder,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo
			opts.Client = shared.DiscussionClientFunc(f)

			if opts.Limit < 1 {
				return cmdutil.FlagErrorf("invalid limit: %v", opts.Limit)
			}

			if runF != nil {
				return runF(opts)
			}
			return listRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Author, "author", "A", "", "Filter by author")
	cmd.Flags().StringVarP(&opts.Category, "category", "c", "", "Filter by category name or slug")
	cmd.Flags().StringSliceVarP(&opts.Labels, "label", "l", nil, "Filter by label")
	cmdutil.StringEnumFlag(cmd, &opts.State, "state", "s", "open", []string{"open", "closed", "all"}, "Filter by state")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of discussions to fetch")
	cmdutil.NilBoolFlag(cmd, &opts.Answered, "answered", "", "Filter by answered state")
	cmdutil.StringEnumFlag(cmd, &opts.Order, "order", "", "updated", []string{"created", "updated"}, "Order by field")
	cmd.Flags().BoolVarP(&opts.WebMode, "web", "w", false, "List discussions in the web browser")
	cmdutil.AddJSONFlags(cmd, &opts.Exporter, shared.DiscussionFields)

	return cmd
}

func listRun(opts *ListOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	if opts.WebMode {
		return openInBrowser(opts, repo)
	}

	dc, err := opts.Client()
	if err != nil {
		return err
	}

	var categoryID string
	var categorySlug string
	if opts.Category != "" {
		categories, err := dc.ListCategories(repo)
		if err != nil {
			return err
		}
		cat, err := matchCategory(opts.Category, categories)
		if err != nil {
			return err
		}
		categoryID = cat.ID
		categorySlug = cat.Slug
	}

	var discussions []client.Discussion
	var totalCount int

	useSearch := opts.Author != "" || len(opts.Labels) > 0
	if useSearch {
		filters := client.SearchFilters{
			Author:   opts.Author,
			Labels:   opts.Labels,
			State:    opts.State,
			Category: categorySlug,
			Answered: opts.Answered,
			OrderBy:  opts.Order,
		}
		discussions, totalCount, err = dc.Search(repo, filters, opts.Limit)
	} else {
		filters := client.ListFilters{
			State:      opts.State,
			CategoryID: categoryID,
			Answered:   opts.Answered,
			OrderBy:    opts.Order,
		}
		discussions, totalCount, err = dc.List(repo, filters, opts.Limit)
	}
	if err != nil {
		return err
	}

	if opts.Exporter != nil {
		envelope := map[string]interface{}{
			"totalCount":  totalCount,
			"discussions": discussions,
		}
		return opts.Exporter.Write(opts.IO, envelope)
	}

	if len(discussions) == 0 {
		return noResults(repo, opts.State)
	}

	if err := opts.IO.StartPager(); err == nil {
		defer opts.IO.StopPager()
	} else {
		fmt.Fprintf(opts.IO.ErrOut, "failed to start pager: %v\n", err)
	}

	isTerminal := opts.IO.IsStdoutTTY()
	if isTerminal {
		title := listHeader(ghrepo.FullName(repo), len(discussions), totalCount, opts.State)
		fmt.Fprintf(opts.IO.Out, "\n%s\n\n", title)
	}

	printDiscussions(opts, discussions, totalCount)
	return nil
}

func openInBrowser(opts *ListOptions, repo ghrepo.Interface) error {
	discussionsURL := ghrepo.GenerateRepoURL(repo, "discussions")

	var queryParts []string
	if opts.State != "" && opts.State != "all" {
		queryParts = append(queryParts, "is:"+opts.State)
	}
	if opts.Author != "" {
		queryParts = append(queryParts, "author:"+opts.Author)
	}
	for _, l := range opts.Labels {
		queryParts = append(queryParts, fmt.Sprintf("label:%q", l))
	}
	if opts.Category != "" {
		queryParts = append(queryParts, fmt.Sprintf("category:%q", opts.Category))
	}
	if opts.Answered != nil {
		if *opts.Answered {
			queryParts = append(queryParts, "is:answered")
		} else {
			queryParts = append(queryParts, "is:unanswered")
		}
	}

	if len(queryParts) > 0 {
		discussionsURL += "?" + url.Values{"q": {strings.Join(queryParts, " ")}}.Encode()
	}

	if opts.IO.IsStderrTTY() {
		fmt.Fprintf(opts.IO.ErrOut, "Opening %s in your browser.\n", text.DisplayURL(discussionsURL))
	}
	return opts.Browser.Browse(discussionsURL)
}

func matchCategory(input string, categories []client.DiscussionCategory) (*client.DiscussionCategory, error) {
	for i := range categories {
		if strings.EqualFold(categories[i].Slug, input) {
			return &categories[i], nil
		}
	}
	for i := range categories {
		if strings.EqualFold(categories[i].Name, input) {
			return &categories[i], nil
		}
	}

	var available strings.Builder
	for _, c := range categories {
		fmt.Fprintf(&available, "  %s (%s)\n", c.Slug, c.Name)
	}
	return nil, fmt.Errorf("category not found: %s\n\nAvailable categories:\n%s", input, available.String())
}

func noResults(repo ghrepo.Interface, state string) error {
	stateQualifier := ""
	switch state {
	case "open":
		stateQualifier = " open"
	case "closed":
		stateQualifier = " closed"
	}
	return cmdutil.NewNoResultsError(fmt.Sprintf("no%s discussions match your search in %s", stateQualifier, ghrepo.FullName(repo)))
}

func listHeader(repoName string, count, total int, state string) string {
	stateQualifier := ""
	switch state {
	case "open":
		stateQualifier = " open"
	case "closed":
		stateQualifier = " closed"
	}
	return fmt.Sprintf("Showing %d of %d%s discussions in %s", count, total, stateQualifier, repoName)
}

func printDiscussions(opts *ListOptions, discussions []client.Discussion, totalCount int) {
	isTerminal := opts.IO.IsStdoutTTY()
	cs := opts.IO.ColorScheme()
	now := opts.Now()

	headers := []string{"ID", "TITLE", "CATEGORY", "LABELS", "ANSWERED", "UPDATED"}
	if !isTerminal {
		headers = []string{"ID", "STATE", "TITLE", "CATEGORY", "LABELS", "ANSWERED", "UPDATED"}
	}
	tp := tableprinter.New(opts.IO, tableprinter.WithHeader(headers...))

	for _, d := range discussions {
		if isTerminal {
			idColor := cs.Green
			if strings.EqualFold(d.State, "CLOSED") {
				idColor = cs.Gray
			}
			tp.AddField(fmt.Sprintf("#%d", d.Number), tableprinter.WithColor(idColor))
		} else {
			tp.AddField(fmt.Sprintf("%d", d.Number))
			tp.AddField(d.State)
		}

		tp.AddField(text.RemoveExcessiveWhitespace(d.Title))
		tp.AddField(d.Category.Name)

		labelNames := make([]string, len(d.Labels))
		for i, l := range d.Labels {
			if isTerminal {
				labelNames[i] = cs.Label(l.Color, l.Name)
			} else {
				labelNames[i] = l.Name
			}
		}
		tp.AddField(strings.Join(labelNames, ", "), tableprinter.WithTruncate(nil))

		if d.Answered {
			tp.AddField("✓")
		} else {
			tp.AddField("")
		}

		tp.AddTimeField(now, d.UpdatedAt, cs.Muted)
		tp.EndRow()
	}

	_ = tp.Render()

	remaining := totalCount - len(discussions)
	if remaining > 0 {
		fmt.Fprintf(opts.IO.Out, cs.Muted("And %d more\n"), remaining)
	}
}
