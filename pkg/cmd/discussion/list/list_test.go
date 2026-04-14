package list

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

func fixedTime() time.Time {
	return time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
}

func sampleDiscussions() []client.Discussion {
	return []client.Discussion{
		{
			Number: 42,
			Title:  "Bug report discussion",
			URL:    "https://github.com/OWNER/REPO/discussions/42",
			Author: client.DiscussionAuthor{Login: "monalisa"},
			Category: client.DiscussionCategory{
				ID:   "CAT1",
				Name: "General",
				Slug: "general",
			},
			Labels: []client.DiscussionLabel{
				{ID: "L1", Name: "bug", Color: "d73a4a"},
			},
			Answered:  true,
			UpdatedAt: time.Date(2025, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			Number: 41,
			Title:  "Feature request",
			URL:    "https://github.com/OWNER/REPO/discussions/41",
			Author: client.DiscussionAuthor{Login: "octocat"},
			Category: client.DiscussionCategory{
				ID:   "CAT2",
				Name: "Ideas",
				Slug: "ideas",
			},
			Labels:    []client.DiscussionLabel{},
			Answered:  false,
			UpdatedAt: time.Date(2025, 2, 20, 12, 0, 0, 0, time.UTC),
		},
	}
}

func sampleResult() client.DiscussionListResult {
	return client.DiscussionListResult{
		Discussions: sampleDiscussions(),
		TotalCount:  2,
	}
}

func sampleCategories() []client.DiscussionCategory {
	return []client.DiscussionCategory{
		{ID: "CAT1", Name: "General", Slug: "general", IsAnswerable: true},
		{ID: "CAT2", Name: "Ideas", Slug: "ideas", IsAnswerable: false},
		{ID: "CAT3", Name: "Show and tell", Slug: "show-and-tell", IsAnswerable: false},
	}
}

func TestListRun_tty(t *testing.T) {
	ios, _, stdout, stderr := iostreams.Test()
	ios.SetStdoutTTY(true)
	ios.SetStderrTTY(true)

	mockClient := &client.DiscussionClientMock{
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			return sampleResult(), nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)

	assert.Equal(t, "", stderr.String())
	out := stdout.String()
	assert.Contains(t, out, "Showing 2 of 2 open discussions in OWNER/REPO")
	assert.Contains(t, out, "#42")
	assert.Contains(t, out, "Bug report discussion")
	assert.Contains(t, out, "General")
	assert.Contains(t, out, "✓")
	assert.Contains(t, out, "#41")
	assert.Contains(t, out, "Feature request")
}

func TestListRun_nontty(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	mockClient := &client.DiscussionClientMock{
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			return sampleResult(), nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.NotContains(t, out, "Showing")
	assert.Contains(t, out, "42")
	assert.Contains(t, out, "OPEN")
	assert.Contains(t, out, "Bug report discussion")
}

func TestListRun_json(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()

	mockClient := &client.DiscussionClientMock{
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			return client.DiscussionListResult{
				Discussions: sampleDiscussions(),
				TotalCount:  2,
				NextCursor:  "CURSOR123",
			}, nil
		},
	}

	exporter := cmdutil.NewJSONExporter()
	exporter.SetFields([]string{"number", "title"})

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Exporter: exporter,
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, `"totalCount"`)
	assert.Contains(t, out, `"discussions"`)
	assert.Contains(t, out, `"next"`)
}

func TestListRun_web(t *testing.T) {
	ios, _, _, stderr := iostreams.Test()
	ios.SetStderrTTY(true)

	br := &browser.Stub{}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Browser:  br,
		WebMode:  true,
		State:    "open",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "Opening")
	assert.Contains(t, br.BrowsedURL(), "github.com/OWNER/REPO/discussions")
}

func TestListRun_noResults(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	mockClient := &client.DiscussionClientMock{
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			return client.DiscussionListResult{}, nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.Error(t, err)
	var noResultsErr cmdutil.NoResultsError
	assert.ErrorAs(t, err, &noResultsErr)
}

func TestListRun_categoryFilter(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	mockClient := &client.DiscussionClientMock{
		ListCategoriesFunc: func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
			return sampleCategories(), nil
		},
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			assert.Equal(t, "CAT1", filters.CategoryID)
			return client.DiscussionListResult{
				Discussions: sampleDiscussions()[:1],
				TotalCount:  1,
			}, nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Category: "general",
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bug report discussion")
}

func TestListRun_categoryNotFound(t *testing.T) {
	ios, _, _, _ := iostreams.Test()

	mockClient := &client.DiscussionClientMock{
		ListCategoriesFunc: func(repo ghrepo.Interface) ([]client.DiscussionCategory, error) {
			return sampleCategories(), nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Category: "nonexistent",
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown category: "nonexistent"`)
}

func TestListRun_authorFilter(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	mockClient := &client.DiscussionClientMock{
		SearchFunc: func(repo ghrepo.Interface, filters client.SearchFilters, after string, limit int) (client.DiscussionListResult, error) {
			assert.Equal(t, "monalisa", filters.Author)
			return client.DiscussionListResult{
				Discussions: sampleDiscussions()[:1],
				TotalCount:  1,
			}, nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Author:   "monalisa",
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bug report discussion")
}

func TestListRun_labelFilter(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	mockClient := &client.DiscussionClientMock{
		SearchFunc: func(repo ghrepo.Interface, filters client.SearchFilters, after string, limit int) (client.DiscussionListResult, error) {
			assert.Equal(t, []string{"bug", "docs"}, filters.Labels)
			return client.DiscussionListResult{
				Discussions: sampleDiscussions()[:1],
				TotalCount:  1,
			}, nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Labels:   []string{"bug", "docs"},
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bug report discussion")
}

func TestListRun_searchFilter(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	mockClient := &client.DiscussionClientMock{
		SearchFunc: func(repo ghrepo.Interface, filters client.SearchFilters, after string, limit int) (client.DiscussionListResult, error) {
			assert.Equal(t, "some keywords", filters.Keywords)
			return client.DiscussionListResult{
				Discussions: sampleDiscussions()[:1],
				TotalCount:  1,
			}, nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		Search:   "some keywords",
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bug report discussion")
}

func TestListRun_afterCursor(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	mockClient := &client.DiscussionClientMock{
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			assert.Equal(t, "CURSOR_ABC", after)
			return sampleResult(), nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		State:    "open",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		After:    "CURSOR_ABC",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)
}

func TestNewCmdList(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantsErr bool
	}{
		{
			name: "no flags",
			args: "",
		},
		{
			name: "state flag",
			args: "--state closed",
		},
		{
			name: "label flag",
			args: "--label bug --label docs",
		},
		{
			name: "author flag",
			args: "--author monalisa",
		},
		{
			name: "category flag",
			args: "--category general",
		},
		{
			name: "limit flag",
			args: "--limit 10",
		},
		{
			name:     "invalid limit",
			args:     "--limit 0",
			wantsErr: true,
		},
		{
			name: "web flag",
			args: "--web",
		},
		{
			name: "sort flag",
			args: "--sort created",
		},
		{
			name: "order flag",
			args: "--order asc",
		},
		{
			name: "sort and order flags",
			args: "--sort created --order asc",
		},
		{
			name: "search flag",
			args: "--search \"some query\"",
		},
		{
			name: "after flag",
			args: "--after CURSOR123",
		},
		{
			name:     "invalid state",
			args:     "--state invalid",
			wantsErr: true,
		},
		{
			name:     "invalid sort",
			args:     "--sort invalid",
			wantsErr: true,
		},
		{
			name:     "invalid order",
			args:     "--order invalid",
			wantsErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: ios,
				Browser:   &browser.Stub{},
				BaseRepo:  func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
			}

			var gotOpts *ListOptions
			cmd := NewCmdList(f, func(o *ListOptions) error {
				gotOpts = o
				return nil
			})

			argv := []string{}
			if tt.args != "" {
				argv = splitArgs(tt.args)
			}
			cmd.SetArgs(argv)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err := cmd.ExecuteC()

			if tt.wantsErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			_ = gotOpts
		})
	}
}

func TestToFilterState(t *testing.T) {
	tests := []struct {
		input string
		want  *string
	}{
		{"open", strPtr(client.FilterStateOpen)},
		{"closed", strPtr(client.FilterStateClosed)},
		{"all", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toFilterState(tt.input)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, *tt.want, *got)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func splitArgs(s string) []string {
	var args []string
	for _, part := range splitRespectingQuotes(s) {
		if part != "" {
			args = append(args, part)
		}
	}
	return args
}

func splitRespectingQuotes(s string) []string {
	var result []string
	var current []byte
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if s[i] == ' ' && !inQuote {
			result = append(result, string(current))
			current = nil
			continue
		}
		current = append(current, s[i])
	}
	result = append(result, string(current))
	return result
}

func TestListRun_closedState(t *testing.T) {
	ios, _, stdout, _ := iostreams.Test()
	ios.SetStdoutTTY(true)

	closed := []client.Discussion{
		{
			Number:    10,
			Title:     "Old discussion",
			Closed:    true,
			Category:  client.DiscussionCategory{Name: "General"},
			Labels:    []client.DiscussionLabel{},
			UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	mockClient := &client.DiscussionClientMock{
		ListFunc: func(repo ghrepo.Interface, filters client.ListFilters, after string, limit int) (client.DiscussionListResult, error) {
			return client.DiscussionListResult{
				Discussions: closed,
				TotalCount:  1,
			}, nil
		},
	}

	opts := &ListOptions{
		IO:       ios,
		BaseRepo: func() (ghrepo.Interface, error) { return ghrepo.New("OWNER", "REPO"), nil },
		Client:   func() (client.DiscussionClient, error) { return mockClient, nil },
		State:    "closed",
		Limit:    30,
		Sort:     "updated",
		Order:    "desc",
		Now:      fixedTime,
	}

	err := listRun(opts)
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "closed discussions")
	assert.Contains(t, out, "Old discussion")
	assert.Contains(t, out, "#10")
}
