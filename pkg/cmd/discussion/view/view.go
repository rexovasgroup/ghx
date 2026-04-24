package view

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/browser"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/text"
	"github.com/cli/cli/v2/pkg/cmd/discussion/client"
	"github.com/cli/cli/v2/pkg/cmd/discussion/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/pkg/markdown"
	"github.com/spf13/cobra"
)

// ViewOptions holds the configuration for the view command.
type ViewOptions struct {
	IO       *iostreams.IOStreams
	BaseRepo func() (ghrepo.Interface, error)
	Browser  browser.Browser
	Client   func() (client.DiscussionClient, error)

	DiscussionNumber int
	WebMode          bool
	Comments         bool
	Order            string
	Exporter         cmdutil.Exporter
	Now              func() time.Time
}

// NewCmdView creates the "discussion view" command.
func NewCmdView(f *cmdutil.Factory, runF func(*ViewOptions) error) *cobra.Command {
	opts := &ViewOptions{
		IO:      f.IOStreams,
		Browser: f.Browser,
		Now:     time.Now,
	}

	cmd := &cobra.Command{
		Use:   "view {<number> | <url>}",
		Short: "View a discussion (preview)",
		Long: heredoc.Docf(`
			Display the title, body, and other information about a discussion.

			With %[1]s--comments%[1]s flag, show threaded comments on the discussion.
			Use %[1]s--order%[1]s to control comment ordering (oldest or newest first).

			With %[1]s--web%[1]s flag, open the discussion in a web browser instead.
		`, "`"),
		Example: heredoc.Doc(`
			# View a discussion by number
			$ gh discussion view 123

			# View a discussion by URL
			$ gh discussion view https://github.com/OWNER/REPO/discussions/123

			# View with comments
			$ gh discussion view 123 --comments

			# View with newest comments first
			$ gh discussion view 123 --comments --order newest

			# Open in browser
			$ gh discussion view 123 --web
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("order") && !opts.Comments {
				return cmdutil.FlagErrorf("--order requires --comments")
			}

			number, repo, err := shared.ParseDiscussionArg(args[0])
			if err != nil {
				return cmdutil.FlagErrorf("%s", err)
			}

			if repo != nil {
				opts.BaseRepo = func() (ghrepo.Interface, error) {
					return repo, nil
				}
			} else {
				opts.BaseRepo = f.BaseRepo
			}

			opts.DiscussionNumber = number
			opts.Client = shared.DiscussionClientFunc(f)

			if runF != nil {
				return runF(opts)
			}
			return viewRun(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.WebMode, "web", "w", false, "Open a discussion in the browser")
	cmd.Flags().BoolVarP(&opts.Comments, "comments", "c", false, "View discussion comments")
	cmdutil.StringEnumFlag(cmd, &opts.Order, "order", "", "oldest", []string{"oldest", "newest"}, "Order of comments")
	cmdutil.AddJSONFlags(cmd, &opts.Exporter, shared.DiscussionFields)

	return cmd
}

func viewRun(opts *ViewOptions) error {
	repo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	if opts.WebMode {
		openURL := ghrepo.GenerateRepoURL(repo, "discussions/%d", opts.DiscussionNumber)
		if opts.IO.IsStdoutTTY() {
			fmt.Fprintf(opts.IO.ErrOut, "Opening %s in your browser.\n", text.DisplayURL(openURL))
		}
		return opts.Browser.Browse(openURL)
	}

	c, err := opts.Client()
	if err != nil {
		return err
	}

	opts.IO.DetectTerminalTheme()
	opts.IO.StartProgressIndicator()

	var discussion *client.Discussion
	if opts.Comments {
		discussion, err = c.GetWithComments(repo, opts.DiscussionNumber, 30, opts.Order)
	} else {
		discussion, err = c.GetByNumber(repo, opts.DiscussionNumber)
	}

	opts.IO.StopProgressIndicator()

	if err != nil {
		return err
	}

	if opts.Exporter != nil {
		return opts.Exporter.Write(opts.IO, discussion)
	}

	if err := opts.IO.StartPager(); err != nil {
		fmt.Fprintf(opts.IO.ErrOut, "error starting pager: %v\n", err)
	}
	defer opts.IO.StopPager()

	if opts.IO.IsStdoutTTY() {
		return printHumanView(opts, discussion)
	}

	return printRawView(opts.IO.Out, discussion, opts.Comments)
}

func printHumanView(opts *ViewOptions, d *client.Discussion) error {
	out := opts.IO.Out
	cs := opts.IO.ColorScheme()

	numberStr := fmt.Sprintf("#%d", d.Number)
	if !d.Closed {
		numberStr = cs.Green(numberStr)
	} else {
		numberStr = cs.Muted(numberStr)
	}
	fmt.Fprintf(out, "%s %s\n", cs.Bold(d.Title), numberStr)

	state := "Open"
	stateColor := cs.Green
	if d.Closed {
		state = "Closed"
		stateColor = cs.Muted
	}

	verb := "Started by"
	if d.Category.IsAnswerable {
		verb = "Asked by"
	}

	fmt.Fprintf(out, "%s · %s · %s %s · %s · %s\n",
		stateColor(state),
		d.Category.Name,
		verb,
		d.Author.Login,
		text.FuzzyAgo(opts.Now(), d.CreatedAt),
		text.Pluralize(d.Comments.TotalCount, "comment"),
	)

	if labels := labelList(d.Labels, cs); labels != "" {
		fmt.Fprint(out, cs.Bold("Labels: "))
		fmt.Fprintln(out, labels)
	}

	var md string
	if d.Body == "" {
		md = fmt.Sprintf("\n  %s\n\n", cs.Muted("No description provided"))
	} else {
		var err error
		md, err = markdown.Render(d.Body,
			markdown.WithTheme(opts.IO.TerminalTheme()),
			markdown.WithWrap(opts.IO.TerminalWidth()))
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "\n%s\n", md)

	if reactions := shared.ReactionGroupList(d.ReactionGroups); reactions != "" {
		fmt.Fprintln(out, reactions)
		fmt.Fprintln(out)
	}

	// Comments section
	if opts.Comments && d.Comments.TotalCount > 0 {
		fmt.Fprintln(out, cs.Bold("Comments"))
		fmt.Fprintln(out)

		for _, c := range d.Comments.Comments {
			if err := printHumanComment(opts, out, c, ""); err != nil {
				return err
			}
		}

		if shown := len(d.Comments.Comments); shown < d.Comments.TotalCount {
			fmt.Fprintf(out, cs.Muted("  And %d more comments\n"), d.Comments.TotalCount-shown)
			fmt.Fprintln(out)
		}
	}

	fmt.Fprintf(out, cs.Muted("View this discussion on GitHub: %s\n"), d.URL)

	return nil
}

func printRawView(out io.Writer, d *client.Discussion, showComments bool) error {
	fmt.Fprintf(out, "title:\t%s\n", d.Title)
	state := "OPEN"
	if d.Closed {
		state = "CLOSED"
	}
	fmt.Fprintf(out, "state:\t%s\n", state)
	fmt.Fprintf(out, "category:\t%s\n", d.Category.Name)
	fmt.Fprintf(out, "author:\t%s\n", d.Author.Login)
	fmt.Fprintf(out, "labels:\t%s\n", labelList(d.Labels, nil))
	fmt.Fprintf(out, "comments:\t%d\n", d.Comments.TotalCount)
	fmt.Fprintf(out, "number:\t%d\n", d.Number)
	fmt.Fprintf(out, "url:\t%s\n", d.URL)
	fmt.Fprintln(out, "--")
	fmt.Fprintln(out, d.Body)

	if showComments {
		for _, c := range d.Comments.Comments {
			printRawComment(out, c, "")
		}
	}

	return nil
}

func printHumanComment(opts *ViewOptions, out io.Writer, c client.DiscussionComment, indent string) error {
	cs := opts.IO.ColorScheme()
	now := opts.Now()

	header := fmt.Sprintf("%s%s commented %s",
		indent,
		cs.Bold(c.Author.Login),
		text.FuzzyAgo(now, c.CreatedAt),
	)
	if c.IsAnswer {
		header += " " + cs.Green("✓ Answer")
	}
	fmt.Fprintln(out, header)

	if c.Body != "" {
		md, err := markdown.Render(c.Body,
			markdown.WithTheme(opts.IO.TerminalTheme()),
			markdown.WithWrap(opts.IO.TerminalWidth()))
		if err != nil {
			return err
		}
		if indent != "" {
			md = text.Indent(md, indent)
		}
		fmt.Fprint(out, md)
	}

	if reactions := shared.ReactionGroupList(c.ReactionGroups); reactions != "" {
		fmt.Fprintf(out, "%s%s\n", indent, reactions)
	}

	fmt.Fprintln(out)

	for _, reply := range c.Replies {
		if err := printHumanComment(opts, out, reply, indent+"  "); err != nil {
			return err
		}
	}

	if shown := len(c.Replies); shown < c.TotalReplies {
		fmt.Fprintf(out, "%s  %s\n\n", indent, cs.Muted(fmt.Sprintf("And %d more replies", c.TotalReplies-shown)))
	}

	return nil
}

func printRawComment(out io.Writer, c client.DiscussionComment, indent string) {
	answer := ""
	if c.IsAnswer {
		answer = "\tanswer"
	}
	fmt.Fprintf(out, "%scomment:\t%s\t%s\t%s%s\n", indent, c.Author.Login, c.CreatedAt.Format(time.RFC3339), c.URL, answer)
	fmt.Fprintf(out, "%s--\n", indent)
	if indent != "" {
		fmt.Fprint(out, text.Indent(c.Body, indent))
	} else {
		fmt.Fprint(out, c.Body)
	}
	fmt.Fprintln(out)

	for _, reply := range c.Replies {
		printRawComment(out, reply, indent+"  ")
	}
}

func labelList(labels []client.DiscussionLabel, cs *iostreams.ColorScheme) string {
	if len(labels) == 0 {
		return ""
	}

	sort.SliceStable(labels, func(i, j int) bool {
		return strings.ToLower(labels[i].Name) < strings.ToLower(labels[j].Name)
	})

	names := make([]string, len(labels))
	for i, l := range labels {
		if cs == nil {
			names[i] = l.Name
		} else {
			names[i] = cs.Label(l.Color, l.Name)
		}
	}
	return strings.Join(names, ", ")
}
