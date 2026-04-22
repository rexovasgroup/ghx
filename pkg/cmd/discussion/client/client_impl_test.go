package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDiscussionClient(reg *httpmock.Registry) DiscussionClient {
	httpClient := &http.Client{}
	httpmock.ReplaceTripper(httpClient, reg)
	return NewDiscussionClient(httpClient)
}

// minimalNode returns a minimal JSON discussion node with the given id and title.
func minimalNode(id, title string) string {
	return `{"id":"` + id + `","number":1,"title":"` + title + `","body":"","url":"","closed":false,"stateReason":"","isAnswered":false,"answerChosenAt":"0001-01-01T00:00:00Z","author":{"__typename":"User","login":"alice"},"category":{"id":"C1","name":"General","slug":"general","emoji":"","isAnswerable":false},"answerChosenBy":null,"labels":{"nodes":[]},"reactionGroups":[],"createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-01-01T00:00:00Z","closedAt":"0001-01-01T00:00:00Z","locked":false}`
}

// listResp builds a mock repository.discussions JSON response.
func listResp(hasNext bool, cursor string, total int, nodes string) string {
	hasNextStr := "false"
	if hasNext {
		hasNextStr = "true"
	}
	return `{"data":{"repository":{"hasDiscussionsEnabled":true,"discussions":{"totalCount":` +
		intStr(total) + `,"pageInfo":{"hasNextPage":` + hasNextStr + `,"endCursor":"` + cursor + `"},"nodes":[` + nodes + `]}}}}`
}

// searchResp builds a mock search JSON response.
func searchResp(hasNext bool, cursor string, count int, nodes string) string {
	hasNextStr := "false"
	if hasNext {
		hasNextStr = "true"
	}
	return `{"data":{"search":{"discussionCount":` +
		intStr(count) + `,"pageInfo":{"hasNextPage":` + hasNextStr + `,"endCursor":"` + cursor + `"},"nodes":[` + nodes + `]}}}`
}

// intStr converts an int to its decimal string representation without importing strconv.
func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	richNode := `{
		"id":"D_rich1","number":42,"title":"Rich discussion","body":"body text here",
		"url":"https://github.com/OWNER/REPO/discussions/42",
		"closed":true,"stateReason":"RESOLVED","isAnswered":true,
		"answerChosenAt":"2024-06-01T12:00:00Z",
		"author":{"__typename":"User","login":"alice","id":"U1","name":"Alice"},
		"category":{"id":"C1","name":"Q&A","slug":"q-a","emoji":":question:","isAnswerable":true},
		"answerChosenBy":{"__typename":"User","login":"bob","id":"U2","name":"Bob"},
		"labels":{"nodes":[{"id":"L1","name":"bug","color":"d73a4a"},{"id":"L2","name":"enhancement","color":"a2eeef"}]},
		"reactionGroups":[],
		"createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-06-02T00:00:00Z",
		"closedAt":"2024-06-01T00:00:00Z","locked":true
	}`

	emptyResp := listResp(false, "", 0, "")
	disabledResp := `{"data":{"repository":{"hasDiscussionsEnabled":false,"discussions":{"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}`

	tests := []struct {
		name       string
		filters    ListFilters
		after      string
		limit      int
		responses  []string
		wantErr    string
		wantTotal  int
		wantLen    int
		wantCursor string
		wantTitles []string
		wantDisc   *Discussion
	}{
		{
			name:      "maps all fields",
			limit:     10,
			responses: []string{listResp(false, "", 1, richNode)},
			wantTotal: 1,
			wantLen:   1,
			wantDisc: &Discussion{
				ID: "D_rich1", Number: 42, Title: "Rich discussion", Body: "body text here",
				URL:         "https://github.com/OWNER/REPO/discussions/42",
				Closed:      true,
				StateReason: "RESOLVED",
				Author:      DiscussionActor{ID: "U1", Login: "alice", Name: "Alice"},
				Category:    DiscussionCategory{ID: "C1", Name: "Q&A", Slug: "q-a", Emoji: ":question:", IsAnswerable: true},
				Labels:      []DiscussionLabel{{ID: "L1", Name: "bug", Color: "d73a4a"}, {ID: "L2", Name: "enhancement", Color: "a2eeef"}},
				Answered:    true,
				AnswerChosenAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
				AnswerChosenBy: &DiscussionActor{ID: "U2", Login: "bob", Name: "Bob"},
				Comments:    DiscussionCommentList{},
				CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC),
				ClosedAt:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
				Locked:      true,
			},
		},
		{
			name:      "discussions disabled",
			limit:     10,
			responses: []string{disabledResp},
			wantErr:   "discussions disabled",
		},
		{
			name:    "limit zero",
			limit:   0,
			wantErr: "limit argument must be positive",
		},
		{
			name:    "invalid orderBy",
			limit:   10,
			filters: ListFilters{OrderBy: "invalid"},
			wantErr: "unknown order-by field",
		},
		{
			name:    "invalid direction",
			limit:   10,
			filters: ListFilters{Direction: "sideways"},
			wantErr: "unknown order direction",
		},
		{
			name:    "invalid state",
			limit:   10,
			filters: ListFilters{State: new("merged")},
			wantErr: "unknown state filter",
		},
		{
			name:      "with after cursor",
			limit:     10,
			after:     "someCursor",
			responses: []string{emptyResp},
		},
		{
			name:      "open state filter",
			limit:     10,
			filters:   ListFilters{State: new(FilterStateOpen)},
			responses: []string{emptyResp},
		},
		{
			name:      "closed state filter",
			limit:     10,
			filters:   ListFilters{State: new(FilterStateClosed)},
			responses: []string{emptyResp},
		},
		{
			name:      "answered filter",
			limit:     10,
			filters:   ListFilters{Answered: new(true)},
			responses: []string{emptyResp},
		},
		{
			name:      "unanswered filter",
			limit:     10,
			filters:   ListFilters{Answered: new(false)},
			responses: []string{emptyResp},
		},
		{
			name:      "category ID filter",
			limit:     10,
			filters:   ListFilters{CategoryID: "CAT123"},
			responses: []string{emptyResp},
		},
		{
			name:      "order by created asc",
			limit:     10,
			filters:   ListFilters{OrderBy: OrderByCreated, Direction: OrderDirectionAsc},
			responses: []string{emptyResp},
		},
		{
			name:      "order by updated desc",
			limit:     10,
			filters:   ListFilters{OrderBy: OrderByUpdated, Direction: OrderDirectionDesc},
			responses: []string{emptyResp},
		},
		{
			// When the page has more items than requested, NextCursor is set.
			name:  "pagination sets next cursor",
			limit: 1,
			responses: []string{
				listResp(true, "cursor42", 5, minimalNode("D1", "Discussion 1")),
			},
			wantLen:    1,
			wantTotal:  5,
			wantCursor: "cursor42",
		},
		{
			// Two pages are fetched when limit exceeds the first page's results.
			name:  "pagination fetches multiple pages",
			limit: 2,
			responses: []string{
				listResp(true, "cursor1", 2, minimalNode("D1", "First")),
				listResp(false, "", 2, minimalNode("D2", "Second")),
			},
			wantLen:    2,
			wantTotal:  2,
			wantTitles: []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			for _, resp := range tt.responses {
				reg.Register(httpmock.GraphQL(`query DiscussionList\b`), httpmock.StringResponse(resp))
			}

			c := newTestDiscussionClient(reg)
			result, err := c.List(repo, tt.filters, tt.after, tt.limit)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTotal, result.TotalCount)
			assert.Len(t, result.Discussions, tt.wantLen)
			assert.Equal(t, tt.wantCursor, result.NextCursor)

			for i, title := range tt.wantTitles {
				assert.Equal(t, title, result.Discussions[i].Title)
			}

			if tt.wantDisc != nil {
				require.NotEmpty(t, result.Discussions)
				assert.Equal(t, *tt.wantDisc, result.Discussions[0])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	richNode := `{
		"id":"D_rich1","number":42,"title":"Rich search result","body":"body text here",
		"url":"https://github.com/OWNER/REPO/discussions/42",
		"closed":true,"stateReason":"RESOLVED","isAnswered":true,
		"answerChosenAt":"2024-06-01T12:00:00Z",
		"author":{"__typename":"User","login":"alice","id":"U1","name":"Alice"},
		"category":{"id":"C1","name":"Q&A","slug":"q-a","emoji":":question:","isAnswerable":true},
		"answerChosenBy":{"__typename":"User","login":"bob","id":"U2","name":"Bob"},
		"labels":{"nodes":[{"id":"L1","name":"bug","color":"d73a4a"}]},
		"reactionGroups":[],
		"createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-06-02T00:00:00Z",
		"closedAt":"2024-06-01T00:00:00Z","locked":true
	}`

	emptyResp := searchResp(false, "", 0, "")

	tests := []struct {
		name       string
		filters    SearchFilters
		after      string
		limit      int
		responses  []string
		wantErr    string
		wantTotal  int
		wantLen    int
		wantCursor string
		wantTitles []string
		wantDisc   *Discussion
	}{
		{
			name:      "maps all fields",
			limit:     10,
			responses: []string{searchResp(false, "", 1, richNode)},
			wantTotal: 1,
			wantLen:   1,
			wantDisc: &Discussion{
				ID: "D_rich1", Number: 42, Title: "Rich search result", Body: "body text here",
				URL:         "https://github.com/OWNER/REPO/discussions/42",
				Closed:      true,
				StateReason: "RESOLVED",
				Author:      DiscussionActor{ID: "U1", Login: "alice", Name: "Alice"},
				Category:    DiscussionCategory{ID: "C1", Name: "Q&A", Slug: "q-a", Emoji: ":question:", IsAnswerable: true},
				Labels:      []DiscussionLabel{{ID: "L1", Name: "bug", Color: "d73a4a"}},
				Answered:    true,
				AnswerChosenAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
				AnswerChosenBy: &DiscussionActor{ID: "U2", Login: "bob", Name: "Bob"},
				Comments:    DiscussionCommentList{},
				CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC),
				ClosedAt:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
				Locked:      true,
			},
		},
		{
			name:    "limit zero",
			limit:   0,
			wantErr: "limit argument must be positive",
		},
		{
			name:    "invalid orderBy",
			limit:   10,
			filters: SearchFilters{OrderBy: "bogus"},
			wantErr: "unknown order-by field",
		},
		{
			name:    "invalid direction",
			limit:   10,
			filters: SearchFilters{Direction: "sideways"},
			wantErr: "unknown order direction",
		},
		{
			name:    "invalid state",
			limit:   10,
			filters: SearchFilters{State: new("merged")},
			wantErr: "unknown state filter",
		},
		{
			name:      "with after cursor",
			limit:     10,
			after:     "someCursor",
			responses: []string{emptyResp},
		},
		{
			name:      "open state filter",
			limit:     10,
			filters:   SearchFilters{State: new(FilterStateOpen)},
			responses: []string{emptyResp},
		},
		{
			name:      "closed state filter",
			limit:     10,
			filters:   SearchFilters{State: new(FilterStateClosed)},
			responses: []string{emptyResp},
		},
		{
			name:      "answered filter",
			limit:     10,
			filters:   SearchFilters{Answered: new(true)},
			responses: []string{emptyResp},
		},
		{
			name:      "unanswered filter",
			limit:     10,
			filters:   SearchFilters{Answered: new(false)},
			responses: []string{emptyResp},
		},
		{
			name:      "author filter",
			limit:     10,
			filters:   SearchFilters{Author: "alice"},
			responses: []string{emptyResp},
		},
		{
			name:      "labels filter",
			limit:     10,
			filters:   SearchFilters{Labels: []string{"bug", "enhancement"}},
			responses: []string{emptyResp},
		},
		{
			name:      "category filter",
			limit:     10,
			filters:   SearchFilters{Category: "Q&A"},
			responses: []string{emptyResp},
		},
		{
			name:      "keywords filter",
			limit:     10,
			filters:   SearchFilters{Keywords: "some keyword"},
			responses: []string{emptyResp},
		},
		{
			name:      "order by created asc",
			limit:     10,
			filters:   SearchFilters{OrderBy: OrderByCreated, Direction: OrderDirectionAsc},
			responses: []string{emptyResp},
		},
		{
			name:      "order by updated desc",
			limit:     10,
			filters:   SearchFilters{OrderBy: OrderByUpdated, Direction: OrderDirectionDesc},
			responses: []string{emptyResp},
		},
		{
			// When the page has more items than requested, NextCursor is set.
			name:  "pagination sets next cursor",
			limit: 1,
			responses: []string{
				searchResp(true, "searchCursor42", 5, minimalNode("D1", "Discussion 1")),
			},
			wantLen:    1,
			wantTotal:  5,
			wantCursor: "searchCursor42",
		},
		{
			// Two pages are fetched when limit exceeds the first page's results.
			name:  "pagination fetches multiple pages",
			limit: 2,
			responses: []string{
				searchResp(true, "searchCursor1", 2, minimalNode("D1", "First")),
				searchResp(false, "", 2, minimalNode("D2", "Second")),
			},
			wantLen:    2,
			wantTotal:  2,
			wantTitles: []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			for _, resp := range tt.responses {
				reg.Register(httpmock.GraphQL(`query DiscussionListSearch\b`), httpmock.StringResponse(resp))
			}

			c := newTestDiscussionClient(reg)
			result, err := c.Search(repo, tt.filters, tt.after, tt.limit)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTotal, result.TotalCount)
			assert.Len(t, result.Discussions, tt.wantLen)
			assert.Equal(t, tt.wantCursor, result.NextCursor)

			for i, title := range tt.wantTitles {
				assert.Equal(t, title, result.Discussions[i].Title)
			}

			if tt.wantDisc != nil {
				require.NotEmpty(t, result.Discussions)
				assert.Equal(t, *tt.wantDisc, result.Discussions[0])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ListCategories
// ---------------------------------------------------------------------------

func TestListCategories(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	tests := []struct {
		name       string
		response   string
		wantErr    string
		wantCats   []DiscussionCategory
	}{
		{
			name: "maps all fields",
			response: `{"data":{"repository":{
				"hasDiscussionsEnabled":true,
				"discussionCategories":{"nodes":[
					{"id":"C1","name":"General","slug":"general","emoji":":speech_balloon:","isAnswerable":false},
					{"id":"C2","name":"Q&A","slug":"q-a","emoji":":question:","isAnswerable":true}
				]}
			}}}`,
			wantCats: []DiscussionCategory{
				{ID: "C1", Name: "General", Slug: "general", Emoji: ":speech_balloon:", IsAnswerable: false},
				{ID: "C2", Name: "Q&A", Slug: "q-a", Emoji: ":question:", IsAnswerable: true},
			},
		},
		{
			name: "discussions disabled",
			response: `{"data":{"repository":{
				"hasDiscussionsEnabled":false,
				"discussionCategories":{"nodes":[]}
			}}}`,
			wantErr: "discussions disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			reg.Register(httpmock.GraphQL(`query DiscussionCategoryList\b`), httpmock.StringResponse(tt.response))

			c := newTestDiscussionClient(reg)
			categories, err := c.ListCategories(repo)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, categories, len(tt.wantCats))
			for i, want := range tt.wantCats {
				assert.Equal(t, want, categories[i])
			}
		})
	}
}
