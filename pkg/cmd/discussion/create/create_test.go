package create

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/pkg/cmd/discussion/client"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleCategories() []client.DiscussionCategory {
	return []client.DiscussionCategory{
		{ID: "CAT1", Name: "General", Slug: "general"},
		{ID: "CAT2", Name: "Q&A", Slug: "q-a"},
		{ID: "CAT3", Name: "Show and tell", Slug: "show-and-tell"},
	}
}

func sampleDiscussion() *client.Discussion {
	return &client.Discussion{
		Number: 5,
		Title:  "My question",
		URL:    "https://github.com/OWNER/REPO/discussions/5",
	}
}

func TestNewCmdCreate(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantOpts CreateOptions
		wantErr  string
	}{
		{
			name:     "no flags",
			args:     "",
			wantOpts: CreateOptions{},
		},
		{
			name: "title flag",
			args: "--title 'My question'",
			wantOpts: CreateOptions{
				Title: "My question",
			},
		},
		{
			name: "all flags",
			args: "--title 'My question' --body 'Details' --category 'Q&A' --label bug",
			wantOpts: CreateOptions{
				Title:    "My question",
				Body:     "Details",
				Category: "Q&A",
				Labels:   []string{"bug"},
			},
		},
		{
			name:    "extra args",
			args:    "extra",
			wantErr: "unknown argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &cmdutil.Factory{}
			var capturedOpts *CreateOptions
			cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
				capturedOpts = opts
				return nil
			})
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			argv, err := shlex.Split(tt.args)
			require.NoError(t, err)
			cmd.SetArgs(argv)

			_, err = cmd.ExecuteC()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOpts.Title, capturedOpts.Title)
			assert.Equal(t, tt.wantOpts.Body, capturedOpts.Body)
			assert.Equal(t, tt.wantOpts.Category, capturedOpts.Category)
			assert.Equal(t, tt.wantOpts.Labels, capturedOpts.Labels)
		})
	}
}

func TestCreateRun_nonInteractive(t *testing.T) {
	tests := []struct {
		name      string
		opts      CreateOptions
		wantErr   string
		wantOut   string
		setupMock func(*client.DiscussionClientMock)
	}{
		{
			name: "creates discussion successfully",
			opts: CreateOptions{
				Title:    "My question",
				Body:     "Details",
				Category: "Q&A",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
				m.CreateFunc = func(repo ghrepo.Interface, input client.CreateDiscussionInput) (*client.Discussion, error) {
					assert.Equal(t, "CAT2", input.CategoryID)
					assert.Equal(t, "My question", input.Title)
					assert.Equal(t, "Details", input.Body)
					return sampleDiscussion(), nil
				}
			},
			wantOut: "https://github.com/OWNER/REPO/discussions/5\n",
		},
		{
			name: "creates with label",
			opts: CreateOptions{
				Title:    "Feature request",
				Body:     "Details",
				Category: "general",
				Labels:   []string{"enhancement"},
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
				m.CreateFunc = func(repo ghrepo.Interface, input client.CreateDiscussionInput) (*client.Discussion, error) {
					assert.Equal(t, []string{"enhancement"}, input.Labels)
					return sampleDiscussion(), nil
				}
			},
			wantOut: "https://github.com/OWNER/REPO/discussions/5\n",
		},
		{
			name: "missing title returns error",
			opts: CreateOptions{
				Body:     "Details",
				Category: "General",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
			},
			wantErr: "--title required when not running interactively",
		},
		{
			name: "missing category returns error",
			opts: CreateOptions{
				Title: "My question",
				Body:  "Details",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
			},
			wantErr: "--category required when not running interactively",
		},
		{
			name: "missing body returns error",
			opts: CreateOptions{
				Title:    "My question",
				Category: "General",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
			},
			wantErr: "--body required when not running interactively",
		},
		{
			name: "unknown category returns error",
			opts: CreateOptions{
				Title:    "My question",
				Body:     "Details",
				Category: "nonexistent",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
			},
			wantErr: `unknown category: "nonexistent"`,
		},
		{
			name: "ListCategories error propagates",
			opts: CreateOptions{
				Title:    "My question",
				Body:     "Details",
				Category: "General",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: "fetching categories: network error",
		},
		{
			name: "Create error propagates",
			opts: CreateOptions{
				Title:    "My question",
				Body:     "Details",
				Category: "General",
			},
			setupMock: func(m *client.DiscussionClientMock) {
				m.ListCategoriesFunc = func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
					return sampleCategories(), nil
				}
				m.CreateFunc = func(repo ghrepo.Interface, input client.CreateDiscussionInput) (*client.Discussion, error) {
					return nil, fmt.Errorf("mutation failed")
				}
			},
			wantErr: "creating discussion: mutation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, stdout, _ := iostreams.Test()
			// non-interactive: no TTY

			mockClient := &client.DiscussionClientMock{}
			tt.setupMock(mockClient)

			opts := tt.opts
			opts.IO = ios
			opts.BaseRepo = func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil }
			opts.Client = func() (client.DiscussionClient, error) { return mockClient, nil }

			err := createRun(&opts)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOut, stdout.String())
		})
	}
}

func TestCreateRun_tty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStdinTTY(true)

	mockClient := &client.DiscussionClientMock{
		ListCategoriesFunc: func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
			return sampleCategories(), nil
		},
		CreateFunc: func(repo ghrepo.Interface, input client.CreateDiscussionInput) (*client.Discussion, error) {
			assert.Equal(t, "My question", input.Title)
			assert.Equal(t, "CAT1", input.CategoryID)
			assert.Equal(t, "Some body text", input.Body)
			return sampleDiscussion(), nil
		},
	}

	pm := &prompter.PrompterMock{
		InputFunc: func(prompt, defaultValue string) (string, error) {
			return "My question", nil
		},
		SelectFunc: func(prompt, defaultValue string, options []string) (int, error) {
			assert.Equal(t, []string{"General", "Q&A", "Show and tell"}, options)
			return 0, nil
		},
		MarkdownEditorFunc: func(prompt, defaultValue string, blankAllowed bool) (string, error) {
			return "Some body text", nil
		},
	}

	opts := &CreateOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Prompter: pm,
	}

	err := createRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created discussion #5")
	assert.Contains(t, stdout.String(), "https://github.com/OWNER/REPO/discussions/5")
}

func TestCreateRun_tty_partialFlags(t *testing.T) {
	// Title and body provided, category via prompt
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStdinTTY(true)

	mockClient := &client.DiscussionClientMock{
		ListCategoriesFunc: func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
			return sampleCategories(), nil
		},
		CreateFunc: func(repo ghrepo.Interface, input client.CreateDiscussionInput) (*client.Discussion, error) {
			assert.Equal(t, "Pre-filled title", input.Title)
			assert.Equal(t, "CAT2", input.CategoryID)
			assert.Equal(t, "Pre-filled body", input.Body)
			return sampleDiscussion(), nil
		},
	}

	pm := &prompter.PrompterMock{
		SelectFunc: func(prompt, defaultValue string, options []string) (int, error) {
			return 1, nil // select Q&A
		},
	}

	opts := &CreateOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Prompter: pm,
		Title:    "Pre-filled title",
		Body:     "Pre-filled body",
	}

	err := createRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created discussion #5")
}
