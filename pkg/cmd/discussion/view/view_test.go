package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/browser"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/discussion/client"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/pkg/jsonfieldstest"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONFields(t *testing.T) {
	jsonfieldstest.ExpectCommandToSupportJSONFields(t, NewCmdView, []string{
		"id",
		"number",
		"title",
		"body",
		"url",
		"closed",
		"state",
		"stateReason",
		"author",
		"category",
		"labels",
		"answered",
		"answerChosenAt",
		"answerChosenBy",
		"comments",
		"reactionGroups",
		"createdAt",
		"updatedAt",
		"closedAt",
		"locked",
	})
}

func TestNewCmdView(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantErr  string
		wantOpts ViewOptions
		wantRepo string
	}{
		{
			name: "number argument",
			args: "123",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Limit:            30,
				Order:            "newest",
			},
		},
		{
			name: "hash number argument",
			args: "'#456'",
			wantOpts: ViewOptions{
				DiscussionNumber: 456,
				Limit:            30,
				Order:            "newest",
			},
		},
		{
			name: "URL argument",
			args: "https://github.com/OTHER/REPO/discussions/789",
			wantOpts: ViewOptions{
				DiscussionNumber: 789,
				Limit:            30,
				Order:            "newest",
			},
			wantRepo: "OTHER/REPO",
		},
		{
			name:    "invalid argument",
			args:    "not-a-number",
			wantErr: "invalid discussion argument",
		},
		{
			name:    "no arguments",
			args:    "",
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name: "web flag",
			args: "123 --web",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				WebMode:          true,
				Limit:            30,
				Order:            "newest",
			},
		},
		{
			name: "comments flag",
			args: "123 --comments",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Comments:         true,
				Limit:            30,
				Order:            "newest",
			},
		},
		{
			name: "comments with limit",
			args: "123 --comments --limit 10",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Comments:         true,
				Limit:            10,
				Order:            "newest",
			},
		},
		{
			name: "comments with after",
			args: "123 --comments --after CURSOR_ABC",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Comments:         true,
				Limit:            30,
				After:            "CURSOR_ABC",
				Order:            "newest",
			},
		},
		{
			name: "comments with order oldest",
			args: "123 --comments --order oldest",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Comments:         true,
				Limit:            30,
				Order:            "oldest",
			},
		},
		{
			name: "replies flag",
			args: "123 --replies DC_abc",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Replies:          "DC_abc",
				Limit:            30,
				Order:            "newest",
			},
		},
		{
			name: "replies with limit",
			args: "123 --replies DC_abc --limit 10",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Replies:          "DC_abc",
				Limit:            10,
				Order:            "newest",
			},
		},
		{
			name: "replies with after",
			args: "123 --replies DC_abc --after CURSOR",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Replies:          "DC_abc",
				Limit:            30,
				After:            "CURSOR",
				Order:            "newest",
			},
		},
		{
			name: "replies with order oldest",
			args: "123 --replies DC_abc --order oldest",
			wantOpts: ViewOptions{
				DiscussionNumber: 123,
				Replies:          "DC_abc",
				Limit:            30,
				Order:            "oldest",
			},
		},
		{
			name:    "replies with comments is mutually exclusive",
			args:    "123 --replies DC_abc --comments",
			wantErr: "specify only one of --comments, --replies, or --web",
		},
		{
			name:    "replies with web is mutually exclusive",
			args:    "123 --replies DC_abc --web",
			wantErr: "specify only one of --comments, --replies, or --web",
		},
		{
			name:    "comments with web is mutually exclusive",
			args:    "123 --comments --web",
			wantErr: "specify only one of --comments, --replies, or --web",
		},
		{
			name:    "order requires comments or replies",
			args:    "123 --order newest",
			wantErr: "--order requires --comments or --replies",
		},
		{
			name:    "limit requires comments or replies",
			args:    "123 --limit 5",
			wantErr: "--limit requires --comments or --replies",
		},
		{
			name:    "after requires comments or replies",
			args:    "123 --after CURSOR",
			wantErr: "--after requires --comments or --replies",
		},
		{
			name:    "invalid limit zero",
			args:    "123 --comments --limit 0",
			wantErr: "invalid limit",
		},
		{
			name:    "invalid limit negative",
			args:    "123 --comments --limit -5",
			wantErr: "invalid limit",
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

			argv, err := shlex.Split(tt.args)
			require.NoError(t, err)
			cmd.SetArgs(argv)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err = cmd.ExecuteC()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			repo, err := gotOpts.BaseRepo()
			require.NoError(t, err)
			if tt.wantRepo != "" {
				assert.Equal(t, tt.wantRepo, ghrepo.FullName(repo))
			}
			assert.Equal(t, tt.wantOpts.DiscussionNumber, gotOpts.DiscussionNumber)
			assert.Equal(t, tt.wantOpts.WebMode, gotOpts.WebMode)
			assert.Equal(t, tt.wantOpts.Comments, gotOpts.Comments)
			assert.Equal(t, tt.wantOpts.Replies, gotOpts.Replies)
			assert.Equal(t, tt.wantOpts.Limit, gotOpts.Limit)
			assert.Equal(t, tt.wantOpts.After, gotOpts.After)
			assert.Equal(t, tt.wantOpts.Order, gotOpts.Order)
		})
	}
}

func TestViewRun(t *testing.T) {
	fixedNow := func() time.Time { return time.Date(2025, 3, 1, 1, 0, 0, 0, time.UTC) }

	tests := []struct {
		name        string
		tty         bool
		clientStub  func(*testing.T, *client.DiscussionClientMock)
		opts        ViewOptions
		wantStdout  string
		wantStderr  string
		wantBrowser string
	}{
		{
			name: "tty",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetByNumberFunc = func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					return exampleAnswerableDiscussion(), nil
				}
			},
			wantStdout: heredoc.Doc(`
				an interesting question #123
				Open · Q&A · Asked by monalisa · about 1 hour ago · 3 comments
				Labels: help-wanted

				
				  about my interesting question                                               
				

				👍 5 • 🚀 2

				View this discussion on GitHub: https://github.com/OWNER/REPO/discussions/123
			`),
		},
		{
			name: "nontty",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetByNumberFunc = func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					return exampleAnswerableDiscussion(), nil
				}
			},
			wantStdout: heredoc.Doc(`
				title:	an interesting question
				state:	OPEN
				category:	Q&A
				author:	monalisa
				labels:	help-wanted
				comments:	3
				number:	123
				url:	https://github.com/OWNER/REPO/discussions/123
				--
				about my interesting question
			`),
		},
		{
			name: "web",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetByNumberFunc = func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					return exampleAnswerableDiscussion(), nil
				}
			},
			opts: ViewOptions{
				WebMode: true,
			},
			wantStderr:  "Opening https://github.com/OWNER/REPO/discussions/123 in your browser.\n",
			wantBrowser: "https://github.com/OWNER/REPO/discussions/123",
		},
		{
			name: "not answerable tty",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetByNumberFunc = func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					return exampleUnanswerableDiscussion(), nil
				}
			},
			wantStdout: heredoc.Doc(`
				a cool discussion #123
				Open · General · Started by monalisa · about 1 hour ago · 3 comments
				Labels: help-wanted

				
				  about my cool idea                                                          
				

				👍 5 • 🚀 2

				View this discussion on GitHub: https://github.com/OWNER/REPO/discussions/123
			`),
		},
		{
			name: "comments tty",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetWithCommentsFunc = func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, 30, commentLimit)
					assert.Equal(t, "", after)
					assert.Equal(t, false, newest)
					return exampleDiscussionWithComments(), nil
				}
			},
			opts: ViewOptions{
				Comments: true,
				Order:    "oldest",
			},
			wantStdout: heredoc.Doc(`
				an interesting question #123
				Open · Q&A · Asked by monalisa · about 1 hour ago · 2 comments
				Labels: help-wanted

				
				  about my interesting question                                               
				

				👍 5 • 🚀 2

				Comments

				octocat commented less than a minute ago ✓ Answer

				  This is a comment                                                           

				👍 3

				  hubot commented less than a minute ago
				  
				    Thanks!                                                                     
				  
				  
				  And 4 more replies

				monalisa commented less than a minute ago

				  Another comment                                                             


				View this discussion on GitHub: https://github.com/OWNER/REPO/discussions/123
			`),
		},
		{
			name: "comments nontty",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetWithCommentsFunc = func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, 30, commentLimit)
					assert.Equal(t, "", after)
					assert.Equal(t, false, newest)
					return exampleDiscussionWithComments(), nil
				}
			},
			opts: ViewOptions{
				Comments: true,
				Order:    "oldest",
			},
			wantStdout: heredoc.Doc(`
				title:	an interesting question
				state:	OPEN
				category:	Q&A
				author:	monalisa
				labels:	help-wanted
				comments:	2
				number:	123
				url:	https://github.com/OWNER/REPO/discussions/123
				--
				about my interesting question
				comment:	octocat	2025-03-02T00:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-1	answer
				--
				This is a comment
				  comment:	hubot	2025-03-02T01:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-2
				  --
				  Thanks!
				comment:	monalisa	2025-03-03T00:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-3
				--
				Another comment
			`),
		},
		{
			name: "comments pagination tty",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				d := exampleDiscussionWithComments()
				d.Comments.NextCursor = "NEXT_CURSOR_123"
				m.GetWithCommentsFunc = func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, 10, commentLimit)
					assert.Equal(t, "CURSOR_ABC", after)
					assert.Equal(t, false, newest)
					return d, nil
				}
			},
			opts: ViewOptions{
				Comments: true,
				Limit:    10,
				After:    "CURSOR_ABC",
				Order:    "oldest",
			},
			wantStdout: heredoc.Doc(`
				an interesting question #123
				Open · Q&A · Asked by monalisa · about 1 hour ago · 2 comments
				Labels: help-wanted

				
				  about my interesting question                                               
				

				👍 5 • 🚀 2

				Comments

				octocat commented less than a minute ago ✓ Answer

				  This is a comment                                                           

				👍 3

				  hubot commented less than a minute ago
				  
				    Thanks!                                                                     
				  
				  
				  And 4 more replies

				monalisa commented less than a minute ago

				  Another comment                                                             


				To see more comments, pass: --after NEXT_CURSOR_123

				View this discussion on GitHub: https://github.com/OWNER/REPO/discussions/123
			`),
		},
		{
			name: "comments pagination nontty",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				d := exampleDiscussionWithComments()
				d.Comments.NextCursor = "NEXT_CURSOR_456"
				m.GetWithCommentsFunc = func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, 30, commentLimit)
					assert.Equal(t, "", after)
					assert.Equal(t, false, newest)
					return d, nil
				}
			},
			opts: ViewOptions{
				Comments: true,
				Order:    "oldest",
			},
			wantStdout: heredoc.Doc(`
				title:	an interesting question
				state:	OPEN
				category:	Q&A
				author:	monalisa
				labels:	help-wanted
				comments:	2
				next:	NEXT_CURSOR_456
				number:	123
				url:	https://github.com/OWNER/REPO/discussions/123
				--
				about my interesting question
				comment:	octocat	2025-03-02T00:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-1	answer
				--
				This is a comment
				  comment:	hubot	2025-03-02T01:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-2
				  --
				  Thanks!
				comment:	monalisa	2025-03-03T00:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-3
				--
				Another comment
			`),
		},
		{
			name: "json without comments field",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetByNumberFunc = func(repo ghrepo.Interface, number int) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					return exampleAnswerableDiscussion(), nil
				}
			},
			opts: ViewOptions{
				Exporter: jsonExporter("title", "url"),
			},
			wantStdout: compactJSON(heredoc.Doc(`
				{
				  "title": "an interesting question",
				  "url": "https://github.com/OWNER/REPO/discussions/123"
				}
			`)),
		},
		{
			name: "json with comments field",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetWithCommentsFunc = func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, 30, commentLimit)
					assert.Equal(t, "", after)
					assert.Equal(t, true, newest)
					return exampleDiscussionWithComments(), nil
				}
			},
			opts: ViewOptions{
				Exporter: jsonExporter("comments"),
			},
			wantStdout: compactJSON(heredoc.Doc(`
				{
				  "comments": {
				    "nodes": [
				      {
				        "author": {"id": "", "login": "octocat", "name": ""},
				        "body": "This is a comment",
				        "createdAt": "2025-03-02T00:00:00Z",
				        "id": "C_1",
				        "isAnswer": true,
				        "reactionGroups": [
				          {"content": "THUMBS_UP", "totalCount": 3}
				        ],
				        "replies": {
				          "nodes": [
				            {
				              "author": {"id": "", "login": "hubot", "name": ""},
				              "body": "Thanks!",
				              "createdAt": "2025-03-02T01:00:00Z",
				              "id": "C_1_R1",
				              "isAnswer": false,
				              "reactionGroups": [],
				              "upvoteCount": 0,
				              "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-2"
				            }
				          ],
				          "totalCount": 5
				        },
				        "upvoteCount": 0,
				        "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-1"
				      },
				      {
				        "author": {"id": "", "login": "monalisa", "name": ""},
				        "body": "Another comment",
				        "createdAt": "2025-03-03T00:00:00Z",
				        "id": "C_2",
				        "isAnswer": false,
				        "reactionGroups": [],
				        "replies": {
				          "nodes": [],
				          "totalCount": 0
				        },
				        "upvoteCount": 0,
				        "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-3"
				      }
				    ],
				    "totalCount": 2
				  }
				}
			`)),
		},
		{
			name: "json with comments field pagination",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetWithCommentsFunc = func(repo ghrepo.Interface, number int, commentLimit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, 30, commentLimit)
					assert.Equal(t, "", after)
					assert.Equal(t, true, newest)
					d := exampleDiscussionWithComments()
					d.Comments.NextCursor = "NEXT_COM_CUR"
					return d, nil
				}
			},
			opts: ViewOptions{
				Exporter: jsonExporter("comments"),
			},
			wantStdout: compactJSON(heredoc.Doc(`
				{
				  "comments": {
				    "next": "NEXT_COM_CUR",
				    "nodes": [
				      {
				        "author": {"id": "", "login": "octocat", "name": ""},
				        "body": "This is a comment",
				        "createdAt": "2025-03-02T00:00:00Z",
				        "id": "C_1",
				        "isAnswer": true,
				        "reactionGroups": [
				          {"content": "THUMBS_UP", "totalCount": 3}
				        ],
				        "replies": {
				          "nodes": [
				            {
				              "author": {"id": "", "login": "hubot", "name": ""},
				              "body": "Thanks!",
				              "createdAt": "2025-03-02T01:00:00Z",
				              "id": "C_1_R1",
				              "isAnswer": false,
				              "reactionGroups": [],
				              "upvoteCount": 0,
				              "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-2"
				            }
				          ],
				          "totalCount": 5
				        },
				        "upvoteCount": 0,
				        "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-1"
				      },
				      {
				        "author": {"id": "", "login": "monalisa", "name": ""},
				        "body": "Another comment",
				        "createdAt": "2025-03-03T00:00:00Z",
				        "id": "C_2",
				        "isAnswer": false,
				        "reactionGroups": [],
				        "replies": {
				          "nodes": [],
				          "totalCount": 0
				        },
				        "upvoteCount": 0,
				        "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-3"
				      }
				    ],
				    "totalCount": 2
				  }
				}
			`)),
		},
		{
			name: "replies tty",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetCommentRepliesFunc = func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, "DC_abc", commentID)
					assert.Equal(t, 30, limit)
					assert.Equal(t, "", after)
					assert.Equal(t, true, newest)
					return exampleDiscussionWithReplies(""), nil
				}
			},
			opts: ViewOptions{
				Replies: "DC_abc",
			},
			wantStdout: heredoc.Doc(`
				octocat commented less than a minute ago ✓ Answer

				  This is the parent comment                                                  

				👍 3

				  hubot commented less than a minute ago
				  
				    First reply                                                                 
				  
				  
				  monalisa commented less than a minute ago
				  
				    Second reply                                                                
				  
				  
			`),
		},
		{
			name: "replies pagination tty",
			tty:  true,
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetCommentRepliesFunc = func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, "DC_abc", commentID)
					assert.Equal(t, 30, limit)
					assert.Equal(t, "", after)
					assert.Equal(t, true, newest)
					return exampleDiscussionWithReplies("NEXT_CUR"), nil
				}
			},
			opts: ViewOptions{
				Replies: "DC_abc",
			},
			wantStdout: heredoc.Doc(`
				octocat commented less than a minute ago ✓ Answer

				  This is the parent comment                                                  

				👍 3

				  hubot commented less than a minute ago
				  
				    First reply                                                                 
				  
				  
				  monalisa commented less than a minute ago
				  
				    Second reply                                                                
				  
				  
				To see more replies, pass: --after NEXT_CUR

			`),
		},
		{
			name: "replies nontty",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetCommentRepliesFunc = func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, "DC_abc", commentID)
					assert.Equal(t, 30, limit)
					assert.Equal(t, "", after)
					assert.Equal(t, false, newest)
					return exampleDiscussionWithReplies(""), nil
				}
			},
			opts: ViewOptions{
				Replies: "DC_abc",
				Order:   "oldest",
			},
			wantStdout: heredoc.Doc(`
				comment:	octocat	2025-03-02T00:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-1	answer
				replies:	2
				--
				This is the parent comment
				  comment:	hubot	2025-03-02T01:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-2
				  --
				  First reply
				  comment:	monalisa	2025-03-02T02:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-3
				  --
				  Second reply
			`),
		},
		{
			name: "replies pagination nontty",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetCommentRepliesFunc = func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, "DC_abc", commentID)
					assert.Equal(t, 30, limit)
					assert.Equal(t, "", after)
					assert.Equal(t, false, newest)
					return exampleDiscussionWithReplies("NEXT_CUR_456"), nil
				}
			},
			opts: ViewOptions{
				Replies: "DC_abc",
				Order:   "oldest",
			},
			wantStdout: heredoc.Doc(`
				comment:	octocat	2025-03-02T00:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-1	answer
				replies:	2
				next:	NEXT_CUR_456
				--
				This is the parent comment
				  comment:	hubot	2025-03-02T01:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-2
				  --
				  First reply
				  comment:	monalisa	2025-03-02T02:00:00Z	https://github.com/OWNER/REPO/discussions/123#discussioncomment-3
				  --
				  Second reply
			`),
		},
		{
			name: "replies json",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetCommentRepliesFunc = func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, "DC_abc", commentID)
					assert.Equal(t, 30, limit)
					assert.Equal(t, "", after)
					assert.Equal(t, true, newest)
					return exampleDiscussionWithReplies(""), nil
				}
			},
			opts: ViewOptions{
				Replies:  "DC_abc",
				Exporter: jsonExporter("comments"),
			},
			wantStdout: compactJSON(heredoc.Doc(`
				{
				  "comments": {
				    "nodes": [
				      {
				        "author": {"id": "", "login": "octocat", "name": ""},
				        "body": "This is the parent comment",
				        "createdAt": "2025-03-02T00:00:00Z",
				        "id": "DC_abc",
				        "isAnswer": true,
				        "reactionGroups": [
				          {"content": "THUMBS_UP", "totalCount": 3}
				        ],
				        "replies": {
				          "nodes": [
				            {
				              "author": {"id": "", "login": "hubot", "name": ""},
				              "body": "First reply",
				              "createdAt": "2025-03-02T01:00:00Z",
				              "id": "R1",
				              "isAnswer": false,
				              "reactionGroups": [],
				              "upvoteCount": 0,
				              "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-2"
				            },
				            {
				              "author": {"id": "", "login": "monalisa", "name": ""},
				              "body": "Second reply",
				              "createdAt": "2025-03-02T02:00:00Z",
				              "id": "R2",
				              "isAnswer": false,
				              "reactionGroups": [],
				              "upvoteCount": 0,
				              "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-3"
				            }
				          ],
				          "totalCount": 2
				        },
				        "upvoteCount": 0,
				        "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-1"
				      }
				    ],
				    "totalCount": 1
				  }
				}
			`)),
		},
		{
			name: "replies json pagination",
			clientStub: func(t *testing.T, m *client.DiscussionClientMock) {
				m.GetCommentRepliesFunc = func(repo ghrepo.Interface, number int, commentID string, limit int, after string, newest bool) (*client.Discussion, error) {
					assert.Equal(t, "OWNER/REPO", ghrepo.FullName(repo))
					assert.Equal(t, 123, number)
					assert.Equal(t, "DC_abc", commentID)
					assert.Equal(t, 30, limit)
					assert.Equal(t, "", after)
					assert.Equal(t, true, newest)
					return exampleDiscussionWithReplies("NEXT_REP_CUR"), nil
				}
			},
			opts: ViewOptions{
				Replies:  "DC_abc",
				Exporter: jsonExporter("comments"),
			},
			wantStdout: compactJSON(heredoc.Doc(`
				{
				  "comments": {
				    "nodes": [
				      {
				        "author": {"id": "", "login": "octocat", "name": ""},
				        "body": "This is the parent comment",
				        "createdAt": "2025-03-02T00:00:00Z",
				        "id": "DC_abc",
				        "isAnswer": true,
				        "reactionGroups": [
				          {"content": "THUMBS_UP", "totalCount": 3}
				        ],
				        "replies": {
				          "next": "NEXT_REP_CUR",
				          "nodes": [
				            {
				              "author": {"id": "", "login": "hubot", "name": ""},
				              "body": "First reply",
				              "createdAt": "2025-03-02T01:00:00Z",
				              "id": "R1",
				              "isAnswer": false,
				              "reactionGroups": [],
				              "upvoteCount": 0,
				              "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-2"
				            },
				            {
				              "author": {"id": "", "login": "monalisa", "name": ""},
				              "body": "Second reply",
				              "createdAt": "2025-03-02T02:00:00Z",
				              "id": "R2",
				              "isAnswer": false,
				              "reactionGroups": [],
				              "upvoteCount": 0,
				              "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-3"
				            }
				          ],
				          "totalCount": 2
				        },
				        "upvoteCount": 0,
				        "url": "https://github.com/OWNER/REPO/discussions/123#discussioncomment-1"
				      }
				    ],
				    "totalCount": 1
				  }
				}
			`)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, stdout, stderr := iostreams.Test()
			ios.SetStdoutTTY(tt.tty)
			ios.SetStderrTTY(tt.tty)

			mock := &client.DiscussionClientMock{}
			tt.clientStub(t, mock)

			b := &browser.Stub{}

			opts := tt.opts
			opts.IO = ios
			opts.BaseRepo = func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil }
			opts.Client = func() (client.DiscussionClient, error) { return mock, nil }
			opts.Browser = b
			opts.DiscussionNumber = 123
			opts.Now = fixedNow
			if opts.Limit == 0 {
				opts.Limit = 30
			}
			if opts.Order == "" {
				opts.Order = "newest"
			}

			err := viewRun(&opts)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStdout, stdout.String())
			assert.Equal(t, tt.wantStderr, stderr.String())
			if tt.wantBrowser != "" {
				b.Verify(t, tt.wantBrowser)
			}
		})
	}
}

func exampleDiscussionWithComments() *client.Discussion {
	d := exampleAnswerableDiscussion()
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

func exampleDiscussionWithReplies(nextCursor string) *client.Discussion {
	d := exampleAnswerableDiscussion()
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

func exampleAnswerableDiscussion() *client.Discussion {
	return &client.Discussion{
		ID:     "D_123",
		Number: 123,
		Title:  "an interesting question",
		Body:   "about my interesting question",
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

func exampleUnanswerableDiscussion() *client.Discussion {
	return &client.Discussion{
		ID:     "D_123",
		Number: 123,
		Title:  "a cool discussion",
		Body:   "about my cool idea",
		URL:    "https://github.com/OWNER/REPO/discussions/123",
		Closed: false,
		Author: client.DiscussionActor{Login: "monalisa"},
		Category: client.DiscussionCategory{
			Name: "General", Slug: "general", IsAnswerable: false,
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

func compactJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		panic(fmt.Sprintf("compactJSON: %v", err))
	}
	return buf.String() + "\n"
}

func jsonExporter(fields ...string) cmdutil.Exporter {
	e := cmdutil.NewJSONExporter()
	e.SetFields(fields)
	return e
}
