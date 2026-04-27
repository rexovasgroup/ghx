package view

import (
	"bytes"
	"testing"
	"time"

	"github.com/cli/cli/v2/internal/browser"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/discussion/client"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDiscussion() *client.Discussion {
	return &client.Discussion{
		ID:     "D_123",
		Number: 123,
		Title:  "How to authenticate with SSO?",
		Body:   "I need help with SSO authentication.",
		URL:    "https://github.com/OWNER/REPO/discussions/123",
		Closed: false,
		Author: client.DiscussionActor{Login: "monalisa"},
		Category: client.DiscussionCategory{
			Name: "Q&A", Slug: "q-a", IsAnswerable: true,
		},
		Labels:   []client.DiscussionLabel{{Name: "help-wanted", Color: "0075ca"}},
		Answered: false,
		Comments: client.DiscussionCommentList{TotalCount: 3},
		ReactionGroups: []client.ReactionGroup{
			{Content: "THUMBS_UP", TotalCount: 5},
			{Content: "ROCKET", TotalCount: 2},
		},
		CreatedAt: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestNewCmdView(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantNum int
		wantErr string
	}{
		{
			name:    "number argument",
			args:    []string{"123"},
			wantNum: 123,
		},
		{
			name:    "hash number argument",
			args:    []string{"#456"},
			wantNum: 456,
		},
		{
			name:    "URL argument",
			args:    []string{"https://github.com/OWNER/REPO/discussions/789"},
			wantNum: 789,
		},
		{
			name:    "invalid argument",
			args:    []string{"not-a-number"},
			wantErr: "invalid discussion argument",
		},
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: "accepts 1 arg(s), received 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &cmdutil.Factory{}
			ios, _, _, _ := iostreams.Test()
			f.IOStreams = ios
			f.BaseRepo = func() (ghrepo.Interface, error) {
				return ghrepo.New("OWNER", "REPO"), nil
			}
			f.Browser = &browser.Stub{}

			var gotOpts *ViewOptions
			cmd := NewCmdView(f, func(opts *ViewOptions) error {
				gotOpts = opts
				return nil
			})

			cmd.SetArgs(tt.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			err := cmd.Execute()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNum, gotOpts.DiscussionNumber)
		})
	}
}

func TestViewRun_tty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	d := testDiscussion()
	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Now:              func() time.Time { return time.Date(2025, 3, 1, 1, 0, 0, 0, time.UTC) },
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "How to authenticate with SSO?")
	assert.Contains(t, out, "#123")
	assert.Contains(t, out, "Q&A")
	assert.Contains(t, out, "Asked by")
	assert.Contains(t, out, "monalisa")
	assert.Contains(t, out, "3 comments")
	assert.Contains(t, out, "help-wanted")
	assert.Contains(t, out, "View this discussion on GitHub")
}

func TestViewRun_nontty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussion()
	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "title:\tHow to authenticate with SSO?")
	assert.Contains(t, out, "state:\tOPEN")
	assert.Contains(t, out, "category:\tQ&A")
	assert.Contains(t, out, "author:\tmonalisa")
	assert.Contains(t, out, "labels:\thelp-wanted")
	assert.Contains(t, out, "number:\t123")
	assert.Contains(t, out, "--")
	assert.Contains(t, out, "I need help with SSO authentication.")
}

func TestViewRun_json(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussionWithComments()
	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	exporter := cmdutil.NewJSONExporter()
	exporter.SetFields(discussionFields)

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Limit:            30,
		Order:            "newest",
		Exporter:         exporter,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, `"title"`)
	assert.Contains(t, out, `"number"`)
	assert.Contains(t, out, "How to authenticate with SSO?")
}

func TestViewRun_web(t *testing.T) {
	ios, _, _, stderr := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	b := &browser.Stub{}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Browser:          b,
		DiscussionNumber: 123,
		WebMode:          true,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	b.Verify(t, "https://github.com/OWNER/REPO/discussions/123")
	assert.Contains(t, stderr.String(), "Opening")
}

func TestViewRun_urlArg(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussion()
	d.URL = "https://github.com/OTHER/REPO/discussions/42"
	d.Number = 42

	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			assert.Equal(t, "OTHER", repo.RepoOwner())
			assert.Equal(t, "REPO", repo.RepoName())
			assert.Equal(t, 42, number)
			return d, nil
		},
	}

	f := &cmdutil.Factory{}
	f.IOStreams = ios
	f.BaseRepo = func() (ghrepo.Interface, error) {
		return ghrepo.New("OWNER", "REPO"), nil
	}
	f.Browser = &browser.Stub{}

	var gotOpts *ViewOptions
	cmd := NewCmdView(f, func(opts *ViewOptions) error {
		gotOpts = opts
		opts.Client = func() (client.DiscussionClient, error) {
			return mock, nil
		}
		return viewRun(opts)
	})

	cmd.SetArgs([]string{"https://github.com/OTHER/REPO/discussions/42"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, 42, gotOpts.DiscussionNumber)

	out := stdout.String()
	assert.Contains(t, out, "number:\t42")
}

func TestViewRun_answerable(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	d := testDiscussion()
	d.Category.IsAnswerable = true

	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Now:              func() time.Time { return time.Date(2025, 3, 1, 1, 0, 0, 0, time.UTC) },
	}

	err := viewRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asked by")
}

func TestViewRun_notAnswerable(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	d := testDiscussion()
	d.Category.Name = "General"
	d.Category.IsAnswerable = false

	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Now:              func() time.Time { return time.Date(2025, 3, 1, 1, 0, 0, 0, time.UTC) },
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "Started by")
	assert.NotContains(t, out, "Asked by")
}

func testDiscussionWithComments() *client.Discussion {
	d := testDiscussion()
	d.Comments = client.DiscussionCommentList{
		TotalCount: 2,
		Comments: []client.DiscussionComment{
			{
				ID:        "C_1",
				URL:       "https://github.com/OWNER/REPO/discussions/123#discussioncomment-1",
				Author:    client.DiscussionActor{Login: "octocat"},
				Body:      "This is a comment",
				CreatedAt: time.Date(2025, 3, 2, 0, 0, 0, 0, time.UTC),
				IsAnswer:  true,
				ReactionGroups: []client.ReactionGroup{
					{Content: "THUMBS_UP", TotalCount: 3},
				},
				Replies: client.DiscussionCommentList{
					TotalCount: 5,
					Comments: []client.DiscussionComment{
						{
							ID:        "C_1_R1",
							URL:       "https://github.com/OWNER/REPO/discussions/123#discussioncomment-2",
							Author:    client.DiscussionActor{Login: "hubot"},
							Body:      "Thanks!",
							CreatedAt: time.Date(2025, 3, 2, 1, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			{
				ID:        "C_2",
				URL:       "https://github.com/OWNER/REPO/discussions/123#discussioncomment-3",
				Author:    client.DiscussionActor{Login: "monalisa"},
				Body:      "Another comment",
				CreatedAt: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	return d
}

func TestViewRun_comments_tty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	d := testDiscussionWithComments()
	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			assert.Equal(t, 30, commentLimit)
			assert.Equal(t, false, newest)
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            30,
		Order:            "oldest",
		Now:              func() time.Time { return time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC) },
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "Comments")
	assert.Contains(t, out, "octocat")
	assert.Contains(t, out, "✓ Answer")
	assert.Contains(t, out, "This is a comment")
	assert.Contains(t, out, "hubot")
	assert.Contains(t, out, "Thanks!")
	assert.Contains(t, out, "And 4 more replies")
	assert.Contains(t, out, "monalisa")
	assert.Contains(t, out, "Another comment")
}

func TestViewRun_comments_nontty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussionWithComments()
	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            30,
		Order:            "oldest",
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "comment:\toctocat\t")
	assert.Contains(t, out, "answer")
	assert.Contains(t, out, "This is a comment")
	assert.Contains(t, out, "comment:\thubot\t")
	assert.Contains(t, out, "comment:\tmonalisa\t")
}

func TestViewRun_comments_json(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussionWithComments()
	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	exporter := cmdutil.NewJSONExporter()
	exporter.SetFields(discussionFields)

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            30,
		Order:            "oldest",
		Exporter:         exporter,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, `"totalCount"`)
	assert.Contains(t, out, `"isAnswer":true`)
	assert.Contains(t, out, `"octocat"`)
}

func TestNewCmdView_orderWithoutComments(t *testing.T) {
	f := &cmdutil.Factory{}
	ios, _, _, _ := iostreams.Test()
	f.IOStreams = ios
	f.BaseRepo = func() (ghrepo.Interface, error) {
		return ghrepo.New("OWNER", "REPO"), nil
	}
	f.Browser = &browser.Stub{}

	cmd := NewCmdView(f, func(opts *ViewOptions) error {
		return nil
	})

	cmd.SetArgs([]string{"123", "--order", "newest"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--order requires --comments")
}

func TestViewRun_noComments_usesGetByNumber(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussion()
	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         false,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	assert.Equal(t, 1, len(mock.GetByNumberCalls()))
	assert.Equal(t, 0, len(mock.GetWithCommentsCalls()))
}

func TestNewCmdView_limitWithoutComments(t *testing.T) {
	f := &cmdutil.Factory{}
	ios, _, _, _ := iostreams.Test()
	f.IOStreams = ios
	f.BaseRepo = func() (ghrepo.Interface, error) {
		return ghrepo.New("OWNER", "REPO"), nil
	}
	f.Browser = &browser.Stub{}

	cmd := NewCmdView(f, func(opts *ViewOptions) error {
		return nil
	})

	cmd.SetArgs([]string{"123", "--limit", "10"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--limit requires --comments")
}

func TestNewCmdView_afterWithoutComments(t *testing.T) {
	f := &cmdutil.Factory{}
	ios, _, _, _ := iostreams.Test()
	f.IOStreams = ios
	f.BaseRepo = func() (ghrepo.Interface, error) {
		return ghrepo.New("OWNER", "REPO"), nil
	}
	f.Browser = &browser.Stub{}

	cmd := NewCmdView(f, func(opts *ViewOptions) error {
		return nil
	})

	cmd.SetArgs([]string{"123", "--after", "CURSOR_ABC"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--after requires --comments")
}

func TestNewCmdView_invalidLimit(t *testing.T) {
	f := &cmdutil.Factory{}
	ios, _, _, _ := iostreams.Test()
	f.IOStreams = ios
	f.BaseRepo = func() (ghrepo.Interface, error) {
		return ghrepo.New("OWNER", "REPO"), nil
	}
	f.Browser = &browser.Stub{}

	cmd := NewCmdView(f, func(opts *ViewOptions) error {
		return nil
	})

	cmd.SetArgs([]string{"123", "--comments", "--limit", "0"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid limit")
}

func TestViewRun_commentsWithPagination_tty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	d := testDiscussionWithComments()
	d.Comments.NextCursor = "NEXT_CURSOR_123"

	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			assert.Equal(t, 10, commentLimit)
			assert.Equal(t, "CURSOR_ABC", after)
			assert.Equal(t, false, newest)
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            10,
		After:            "CURSOR_ABC",
		Order:            "oldest",
		Now:              func() time.Time { return time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC) },
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "To see more comments, pass: --after NEXT_CURSOR_123")
}

func TestViewRun_commentsWithPagination_nontty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussionWithComments()
	d.Comments.NextCursor = "NEXT_CURSOR_456"

	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            30,
		Order:            "oldest",
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "next:\tNEXT_CURSOR_456")
}

func TestViewRun_commentsWithPagination_json(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussionWithComments()
	d.Comments.Cursor = "PREV_CURSOR"
	d.Comments.NextCursor = "NEXT_CURSOR_789"

	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	exporter := cmdutil.NewJSONExporter()
	exporter.SetFields(discussionFields)

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            30,
		Order:            "oldest",
		Exporter:         exporter,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, `"cursor":"PREV_CURSOR"`)
	assert.Contains(t, out, `"next":"NEXT_CURSOR_789"`)
}

func TestViewRun_noPaginationCursor_tty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	d := testDiscussionWithComments()

	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         true,
		Limit:            30,
		Order:            "oldest",
		Now:              func() time.Time { return time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC) },
	}

	err := viewRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.NotContains(t, out, "--after")
}

func TestViewRun_jsonComments_usesGetWithComments(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussionWithComments()
	mock := &client.DiscussionClientMock{
		GetWithCommentsFunc: func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
			return d, nil
		},
	}

	exporter := cmdutil.NewJSONExporter()
	exporter.SetFields([]string{"comments"})

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         false,
		Limit:            30,
		Order:            "newest",
		Exporter:         exporter,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	// --json comments should use GetWithComments even without --comments flag
	assert.Equal(t, 0, len(mock.GetByNumberCalls()))
	assert.Equal(t, 1, len(mock.GetWithCommentsCalls()))

	out := stdout.String()
	assert.Contains(t, out, `"totalCount"`)
	assert.Contains(t, out, `"octocat"`)
}

func TestViewRun_jsonWithoutComments_usesGetByNumber(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	d := testDiscussion()
	mock := &client.DiscussionClientMock{
		GetByNumberFunc: func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
			return d, nil
		},
	}

	exporter := cmdutil.NewJSONExporter()
	exporter.SetFields([]string{"title", "number"})

	opts := &ViewOptions{
		IO: ios,
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Client: func() (client.DiscussionClient, error) {
			return mock, nil
		},
		DiscussionNumber: 123,
		Comments:         false,
		Exporter:         exporter,
		Now:              time.Now,
	}

	err := viewRun(opts)
	require.NoError(t, err)

	// --json title,number should NOT fetch comments
	assert.Equal(t, 1, len(mock.GetByNumberCalls()))
	assert.Equal(t, 0, len(mock.GetWithCommentsCalls()))
}

// ---------------------------------------------------------------------------
// --replies flag validation
// ---------------------------------------------------------------------------

func TestNewCmdView_repliesFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "replies with comments is mutually exclusive",
			args:    []string{"123", "--replies", "DC_abc", "--comments"},
			wantErr: "specify only one of --comments, --replies, or --web",
		},
		{
			name:    "replies with web is mutually exclusive",
			args:    []string{"123", "--replies", "DC_abc", "--web"},
			wantErr: "specify only one of --comments, --replies, or --web",
		},
		{
			name:    "order requires comments or replies",
			args:    []string{"123", "--order", "newest"},
			wantErr: "--order requires --comments or --replies",
		},
		{
			name:    "limit requires comments or replies",
			args:    []string{"123", "--limit", "5"},
			wantErr: "--limit requires --comments or --replies",
		},
		{
			name:    "after requires comments or replies",
			args:    []string{"123", "--after", "CURSOR"},
			wantErr: "--after requires --comments or --replies",
		},
		{
			name: "order works with replies",
			args: []string{"123", "--replies", "DC_abc", "--order", "oldest"},
		},
		{
			name: "limit works with replies",
			args: []string{"123", "--replies", "DC_abc", "--limit", "10"},
		},
		{
			name: "after works with replies",
			args: []string{"123", "--replies", "DC_abc", "--after", "CURSOR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &cmdutil.Factory{}
			ios, _, _, _ := iostreams.Test()
			f.IOStreams = ios
			f.BaseRepo = func() (ghrepo.Interface, error) {
				return ghrepo.New("OWNER", "REPO"), nil
			}
			f.Browser = &browser.Stub{}

			cmd := NewCmdView(f, func(opts *ViewOptions) error {
				return nil
			})

			cmd.SetArgs(tt.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			err := cmd.Execute()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// --replies viewRun tests (table-driven)
// ---------------------------------------------------------------------------

func testDiscussionWithReplies(nextCursor string) *client.Discussion {
	d := testDiscussion()
	d.Comments = client.DiscussionCommentList{
		TotalCount: 1,
		Comments: []client.DiscussionComment{
			{
				ID:        "DC_abc",
				URL:       "https://github.com/OWNER/REPO/discussions/123#discussioncomment-1",
				Author:    client.DiscussionActor{Login: "octocat"},
				Body:      "This is the parent comment",
				CreatedAt: time.Date(2025, 3, 2, 0, 0, 0, 0, time.UTC),
				IsAnswer:  true,
				ReactionGroups: []client.ReactionGroup{
					{Content: "THUMBS_UP", TotalCount: 3},
				},
				Replies: client.DiscussionCommentList{
					TotalCount: 2,
					NextCursor: nextCursor,
					Comments: []client.DiscussionComment{
						{
							ID:        "R1",
							URL:       "https://github.com/OWNER/REPO/discussions/123#discussioncomment-2",
							Author:    client.DiscussionActor{Login: "hubot"},
							Body:      "First reply",
							CreatedAt: time.Date(2025, 3, 2, 1, 0, 0, 0, time.UTC),
						},
						{
							ID:        "R2",
							URL:       "https://github.com/OWNER/REPO/discussions/123#discussioncomment-3",
							Author:    client.DiscussionActor{Login: "monalisa"},
							Body:      "Second reply",
							CreatedAt: time.Date(2025, 3, 2, 2, 0, 0, 0, time.UTC),
						},
					},
				},
			},
		},
	}
	return d
}

func TestViewRun_replies(t *testing.T) {
	tests := []struct {
		name         string
		tty          bool
		replies      string
		limit        int
		after        string
		order        string
		exporter     cmdutil.Exporter
		nextCursor   string
		wantContains []string
		wantExcludes []string
		wantClient   func(*testing.T, *client.DiscussionClientMock)
	}{
		{
			name:    "tty renders comment and replies",
			tty:     true,
			replies: "DC_abc",
			limit:   30,
			order:   "newest",
			wantContains: []string{
				"octocat",
				"This is the parent comment",
				"✓ Answer",
				"hubot",
				"First reply",
				"monalisa",
				"Second reply",
			},
		},
		{
			name:       "tty shows pagination hint",
			tty:        true,
			replies:    "DC_abc",
			limit:      30,
			order:      "newest",
			nextCursor: "NEXT_CUR",
			wantContains: []string{
				"--after NEXT_CUR",
			},
		},
		{
			name:    "tty no pagination hint when no next cursor",
			tty:     true,
			replies: "DC_abc",
			limit:   30,
			order:   "newest",
			wantExcludes: []string{
				"--after",
			},
		},
		{
			name:    "nontty raw output",
			tty:     false,
			replies: "DC_abc",
			limit:   30,
			order:   "oldest",
			wantContains: []string{
				"comment:\toctocat\t",
				"answer",
				"replies:\t2",
				"This is the parent comment",
				"hubot",
				"First reply",
			},
		},
		{
			name:       "nontty shows next cursor",
			tty:        false,
			replies:    "DC_abc",
			limit:      30,
			order:      "oldest",
			nextCursor: "NEXT_CUR_456",
			wantContains: []string{
				"next:\tNEXT_CUR_456",
			},
		},
		{
			name:    "json output",
			tty:     false,
			replies: "DC_abc",
			limit:   30,
			order:   "newest",
			exporter: func() cmdutil.Exporter {
				e := cmdutil.NewJSONExporter()
				e.SetFields(discussionFields)
				return e
			}(),
			wantContains: []string{
				`"totalCount"`,
				`"isAnswer":true`,
				`"octocat"`,
			},
		},
		{
			name:    "routes to GetCommentReplies only",
			tty:     false,
			replies: "DC_abc",
			limit:   10,
			after:   "CUR_A",
			order:   "oldest",
			wantClient: func(t *testing.T, mock *client.DiscussionClientMock) {
				require.Equal(t, 1, len(mock.GetCommentRepliesCalls()))
				assert.Equal(t, 0, len(mock.GetByNumberCalls()))
				assert.Equal(t, 0, len(mock.GetWithCommentsCalls()))

				call := mock.GetCommentRepliesCalls()[0]
				assert.Equal(t, "DC_abc", call.CommentID)
				assert.Equal(t, 10, call.Limit)
				assert.Equal(t, "CUR_A", call.After)
				assert.Equal(t, false, call.Newest)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, stdout, _ := iostreams.Test()
			ios.SetStdoutTTY(tt.tty)
			ios.SetStderrTTY(tt.tty)

			d := testDiscussionWithReplies(tt.nextCursor)
			mock := &client.DiscussionClientMock{
				GetCommentRepliesFunc: func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					return d, nil
				},
			}

			opts := &ViewOptions{
				IO: ios,
				BaseRepo: func() (ghrepo.Interface, error) {
					return ghrepo.New("OWNER", "REPO"), nil
				},
				Client: func() (client.DiscussionClient, error) {
					return mock, nil
				},
				DiscussionNumber: 123,
				Replies:          tt.replies,
				Limit:            tt.limit,
				After:            tt.after,
				Order:            tt.order,
				Exporter:         tt.exporter,
				Now:              func() time.Time { return time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC) },
			}

			err := viewRun(opts)
			require.NoError(t, err)

			out := stdout.String()
			for _, s := range tt.wantContains {
				assert.Contains(t, out, s)
			}
			for _, s := range tt.wantExcludes {
				assert.NotContains(t, out, s)
			}
			if tt.wantClient != nil {
				tt.wantClient(t, mock)
			}
		})
	}
}
