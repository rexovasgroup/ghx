package client

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc"
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
	return heredoc.Docf(`
		{
			"id": %q,
			"number": 1,
			"title": %q,
			"body": "",
			"url": "",
			"closed": false,
			"stateReason": "",
			"isAnswered": false,
			"answerChosenAt": "0001-01-01T00:00:00Z",
			"author": {
				"__typename": "User",
				"login": "alice"
			},
			"category": {
				"id": "C1",
				"name": "General",
				"slug": "general",
				"emoji": "",
				"isAnswerable": false
			},
			"answerChosenBy": null,
			"labels": {
				"nodes": []
			},
			"reactionGroups": [],
			"createdAt": "2024-01-01T00:00:00Z",
			"updatedAt": "2024-01-01T00:00:00Z",
			"closedAt": "0001-01-01T00:00:00Z",
			"locked": false
		}
	`, id, title)
}

// minimalNodes returns count comma-separated minimal JSON discussion nodes.
func minimalNodes(count int) string {
	nodes := make([]string, count)
	for i := range nodes {
		nodes[i] = minimalNode(fmt.Sprintf("D%d", i+1), fmt.Sprintf("Discussion %d", i+1))
	}
	return strings.Join(nodes, ",")
}

// listResp builds a mock repository.discussions JSON response.
func listResp(hasNext bool, cursor string, total int, nodes string) string {
	return heredoc.Docf(`
		{
			"data": {
				"repository": {
					"hasDiscussionsEnabled": true,
					"discussions": {
						"totalCount": %d,
						"pageInfo": {
							"hasNextPage": %t,
							"endCursor": %q
						},
						"nodes": [%s]
					}
				}
			}
		}
	`, total, hasNext, cursor, nodes)
}

// searchResp builds a mock search JSON response.
func searchResp(hasNext bool, cursor string, count int, nodes string) string {
	return heredoc.Docf(`
		{
			"data": {
				"search": {
					"discussionCount": %d,
					"pageInfo": {
						"hasNextPage": %t,
						"endCursor": %q
					},
					"nodes": [%s]
				}
			}
		}
	`, count, hasNext, cursor, nodes)
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	richNode := heredoc.Doc(`
		{
			"id": "D_rich1",
			"number": 42,
			"title": "Rich discussion",
			"body": "body text here",
			"url": "https://github.com/OWNER/REPO/discussions/42",
			"closed": true,
			"stateReason": "RESOLVED",
			"isAnswered": true,
			"answerChosenAt": "2024-06-01T12:00:00Z",
			"author": {
				"__typename": "User",
				"login": "alice",
				"id": "U1",
				"name": "Alice"
			},
			"category": {
				"id": "C1",
				"name": "Q&A",
				"slug": "q-a",
				"emoji": ":question:",
				"isAnswerable": true
			},
			"answerChosenBy": {
				"__typename": "User",
				"login": "bob",
				"id": "U2",
				"name": "Bob"
			},
			"labels": {
				"nodes": [
					{"id": "L1", "name": "bug", "color": "d73a4a"},
					{"id": "L2", "name": "enhancement", "color": "a2eeef"}
				]
			},
			"reactionGroups": [],
			"createdAt": "2024-01-01T00:00:00Z",
			"updatedAt": "2024-06-02T00:00:00Z",
			"closedAt": "2024-06-01T00:00:00Z",
			"locked": true
		}
	`)

	emptyResp := listResp(false, "", 0, "")
	disabledResp := heredoc.Doc(`
		{
			"data": {
				"repository": {
					"hasDiscussionsEnabled": false,
					"discussions": {
						"totalCount": 0,
						"pageInfo": {
							"hasNextPage": false,
							"endCursor": null
						},
						"nodes": []
					}
				}
			}
		}
	`)

	tests := []struct {
		name           string
		filters        ListFilters
		after          string
		limit          int
		httpStubs      func(*testing.T, *httpmock.Registry)
		wantErr        string
		wantTotal      int
		wantLen        int
		wantCursor     string
		wantTitles     []string
		wantSingleDisc *Discussion
	}{
		{
			name:  "maps all fields",
			limit: 10,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(listResp(false, "", 1, richNode)),
				)
			},
			wantTotal: 1,
			wantLen:   1,
			wantSingleDisc: &Discussion{
				ID:          "D_rich1",
				Number:      42,
				Title:       "Rich discussion",
				Body:        "body text here",
				URL:         "https://github.com/OWNER/REPO/discussions/42",
				Closed:      true,
				StateReason: "RESOLVED",
				Author: DiscussionActor{
					ID:    "U1",
					Login: "alice",
					Name:  "Alice",
				},
				Category: DiscussionCategory{
					ID:           "C1",
					Name:         "Q&A",
					Slug:         "q-a",
					Emoji:        ":question:",
					IsAnswerable: true,
				},
				Labels: []DiscussionLabel{
					{ID: "L1", Name: "bug", Color: "d73a4a"},
					{ID: "L2", Name: "enhancement", Color: "a2eeef"},
				},
				Answered:       true,
				AnswerChosenAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
				AnswerChosenBy: &DiscussionActor{
					ID:    "U2",
					Login: "bob",
					Name:  "Bob",
				},
				Comments:  DiscussionCommentList{},
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC),
				ClosedAt:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
				Locked:    true,
			},
		},
		{
			name:  "empty list",
			limit: 10,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(emptyResp),
				)
			},
			wantTotal: 0,
			wantLen:   0,
		},
		{
			name:  "discussions disabled",
			limit: 10,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(disabledResp),
				)
			},
			wantErr: "discussions disabled",
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
			name:  "with after cursor",
			limit: 10,
			after: "someCursor",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, "someCursor", vars["after"])
					}),
				)
			},
		},
		{
			name:    "open state filter",
			limit:   10,
			filters: ListFilters{State: new(FilterStateOpen)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, []interface{}{"OPEN"}, vars["states"])
					}),
				)
			},
		},
		{
			name:    "closed state filter",
			limit:   10,
			filters: ListFilters{State: new(FilterStateClosed)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, []interface{}{"CLOSED"}, vars["states"])
					}),
				)
			},
		},
		{
			name:    "answered filter",
			limit:   10,
			filters: ListFilters{Answered: new(true)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, true, vars["answered"])
					}),
				)
			},
		},
		{
			name:    "unanswered filter",
			limit:   10,
			filters: ListFilters{Answered: new(false)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, false, vars["answered"])
					}),
				)
			},
		},
		{
			name:    "category ID filter",
			limit:   10,
			filters: ListFilters{CategoryID: "CAT123"},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, "CAT123", vars["categoryId"])
					}),
				)
			},
		},
		{
			name:    "order by created asc",
			limit:   10,
			filters: ListFilters{OrderBy: OrderByCreated, Direction: OrderDirectionAsc},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						orderBy, ok := vars["orderBy"].(map[string]interface{})
						require.True(t, ok, "orderBy should be a map")
						assert.Equal(t, "CREATED_AT", orderBy["field"])
						assert.Equal(t, "ASC", orderBy["direction"])
					}),
				)
			},
		},
		{
			name:    "order by updated desc",
			limit:   10,
			filters: ListFilters{OrderBy: OrderByUpdated, Direction: OrderDirectionDesc},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						orderBy, ok := vars["orderBy"].(map[string]interface{})
						require.True(t, ok, "orderBy should be a map")
						assert.Equal(t, "UPDATED_AT", orderBy["field"])
						assert.Equal(t, "DESC", orderBy["direction"])
					}),
				)
			},
		},
		{
			// Bot actors have no name; ID comes from the Bot.ID field.
			name:  "bot actor",
			limit: 10,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(listResp(false, "", 1, heredoc.Doc(`
						{
							"id": "D_bot",
							"number": 1,
							"title": "Bot post",
							"body": "",
							"url": "",
							"closed": false,
							"stateReason": "",
							"isAnswered": false,
							"answerChosenAt": "0001-01-01T00:00:00Z",
							"author": {
								"__typename": "Bot",
								"login": "gh-bot",
								"id": "bot-node-id"
							},
							"category": {
								"id": "C1",
								"name": "General",
								"slug": "general",
								"emoji": "",
								"isAnswerable": false
							},
							"answerChosenBy": null,
							"labels": {
								"nodes": []
							},
							"reactionGroups": [],
							"createdAt": "2024-01-01T00:00:00Z",
							"updatedAt": "2024-01-01T00:00:00Z",
							"closedAt": "0001-01-01T00:00:00Z",
							"locked": false
						}
					`))),
				)
			},
			wantLen:   1,
			wantTotal: 1,
			wantSingleDisc: &Discussion{
				ID:        "D_bot",
				Number:    1,
				Title:     "Bot post",
				Author:    DiscussionActor{ID: "bot-node-id", Login: "gh-bot", Name: ""},
				Category:  DiscussionCategory{ID: "C1", Name: "General", Slug: "general"},
				Labels:    []DiscussionLabel{},
				Comments:  DiscussionCommentList{},
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			// When limit > 100, the first page requests 100 and the second page
			// requests the remainder, exercising the per-iteration first variable.
			name:  "limit greater than 100",
			limit: 101,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(listResp(true, "pg2cursor", 101, minimalNodes(100)), func(_ string, vars map[string]interface{}) {
						assert.Equal(t, float64(100), vars["first"])
					}),
				)
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.GraphQLQuery(listResp(false, "", 101, minimalNode("D101", "Discussion 101")), func(_ string, vars map[string]interface{}) {
						assert.Equal(t, float64(1), vars["first"])
					}),
				)
			},
			wantLen:   101,
			wantTotal: 101,
		},
		{
			// When the page has more items than requested, NextCursor is set.
			name:  "pagination sets next cursor",
			limit: 1,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(listResp(true, "cursor42", 5, minimalNode("D1", "Discussion 1"))),
				)
			},
			wantLen:    1,
			wantTotal:  5,
			wantCursor: "cursor42",
		},
		{
			// Two pages are fetched when limit exceeds the first page's results.
			name:  "pagination fetches multiple pages",
			limit: 2,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(listResp(true, "cursor1", 2, minimalNode("D1", "First"))),
				)
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(listResp(false, "", 2, minimalNode("D2", "Second"))),
				)
			},
			wantLen:    2,
			wantTotal:  2,
			wantTitles: []string{"First", "Second"},
		},
		{
			name:  "exact fit does not overfetch",
			limit: 1,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionList\b`),
					httpmock.StringResponse(listResp(false, "", 1, minimalNode("D1", "Only one"))),
				)
			},
			wantLen:    1,
			wantTotal:  1,
			wantTitles: []string{"Only one"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
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

			if tt.wantSingleDisc != nil {
				require.NotEmpty(t, result.Discussions)
				assert.Equal(t, *tt.wantSingleDisc, result.Discussions[0])
			}
		})
	}
}

func TestSearch(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	richNode := heredoc.Doc(`
		{
			"id": "D_rich1",
			"number": 42,
			"title": "Rich search result",
			"body": "body text here",
			"url": "https://github.com/OWNER/REPO/discussions/42",
			"closed": true,
			"stateReason": "RESOLVED",
			"isAnswered": true,
			"answerChosenAt": "2024-06-01T12:00:00Z",
			"author": {
				"__typename": "User",
				"login": "alice",
				"id": "U1",
				"name": "Alice"
			},
			"category": {
				"id": "C1",
				"name": "Q&A",
				"slug": "q-a",
				"emoji": ":question:",
				"isAnswerable": true
			},
			"answerChosenBy": {
				"__typename": "User",
				"login": "bob",
				"id": "U2",
				"name": "Bob"
			},
			"labels": {
				"nodes": [
					{"id": "L1", "name": "bug", "color": "d73a4a"}
				]
			},
			"reactionGroups": [],
			"createdAt": "2024-01-01T00:00:00Z",
			"updatedAt": "2024-06-02T00:00:00Z",
			"closedAt": "2024-06-01T00:00:00Z",
			"locked": true
		}
	`)

	emptyResp := searchResp(false, "", 0, "")

	tests := []struct {
		name           string
		filters        SearchFilters
		after          string
		limit          int
		httpStubs      func(*testing.T, *httpmock.Registry)
		wantErr        string
		wantTotal      int
		wantLen        int
		wantCursor     string
		wantTitles     []string
		wantSingleDisc *Discussion
	}{
		{
			name:  "maps all fields",
			limit: 10,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.StringResponse(searchResp(false, "", 1, richNode)),
				)
			},
			wantTotal: 1,
			wantLen:   1,
			wantSingleDisc: &Discussion{
				ID:          "D_rich1",
				Number:      42,
				Title:       "Rich search result",
				Body:        "body text here",
				URL:         "https://github.com/OWNER/REPO/discussions/42",
				Closed:      true,
				StateReason: "RESOLVED",
				Author: DiscussionActor{
					ID:    "U1",
					Login: "alice",
					Name:  "Alice",
				},
				Category: DiscussionCategory{
					ID:           "C1",
					Name:         "Q&A",
					Slug:         "q-a",
					Emoji:        ":question:",
					IsAnswerable: true,
				},
				Labels: []DiscussionLabel{
					{ID: "L1", Name: "bug", Color: "d73a4a"},
				},
				Answered:       true,
				AnswerChosenAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
				AnswerChosenBy: &DiscussionActor{
					ID:    "U2",
					Login: "bob",
					Name:  "Bob",
				},
				Comments:  DiscussionCommentList{},
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC),
				ClosedAt:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
				Locked:    true,
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
			name:  "with after cursor",
			limit: 10,
			after: "someCursor",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Equal(t, "someCursor", vars["after"])
					}),
				)
			},
		},
		{
			name:    "open state filter",
			limit:   10,
			filters: SearchFilters{State: new(FilterStateOpen)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "is:open")
					}),
				)
			},
		},
		{
			name:    "closed state filter",
			limit:   10,
			filters: SearchFilters{State: new(FilterStateClosed)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "is:closed")
					}),
				)
			},
		},
		{
			name:    "answered filter",
			limit:   10,
			filters: SearchFilters{Answered: new(true)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "is:answered")
					}),
				)
			},
		},
		{
			name:    "unanswered filter",
			limit:   10,
			filters: SearchFilters{Answered: new(false)},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "is:unanswered")
					}),
				)
			},
		},
		{
			name:    "author filter",
			limit:   10,
			filters: SearchFilters{Author: "alice"},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), `author:"alice"`)
					}),
				)
			},
		},
		{
			name:    "labels filter",
			limit:   10,
			filters: SearchFilters{Labels: []string{"bug", "enhancement"}},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						q := vars["query"].(string)
						assert.Contains(t, q, `label:"bug"`)
						assert.Contains(t, q, `label:"enhancement"`)
					}),
				)
			},
		},
		{
			name:    "category filter",
			limit:   10,
			filters: SearchFilters{Category: "Q&A"},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), `category:"Q&A"`)
					}),
				)
			},
		},
		{
			name:    "keywords filter",
			limit:   10,
			filters: SearchFilters{Keywords: "some keyword"},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "some keyword")
					}),
				)
			},
		},
		{
			name:    "order by created asc",
			limit:   10,
			filters: SearchFilters{OrderBy: OrderByCreated, Direction: OrderDirectionAsc},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "sort:created-asc")
					}),
				)
			},
		},
		{
			name:    "order by updated desc",
			limit:   10,
			filters: SearchFilters{OrderBy: OrderByUpdated, Direction: OrderDirectionDesc},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(emptyResp, func(_ string, vars map[string]interface{}) {
						assert.Contains(t, vars["query"].(string), "sort:updated-desc")
					}),
				)
			},
		},
		{
			// When limit > 100, the first page requests 100 and the second page
			// requests the remainder, exercising the per-iteration first variable.
			name:  "limit greater than 100",
			limit: 101,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(searchResp(true, "pg2cursor", 101, minimalNodes(100)), func(_ string, vars map[string]interface{}) {
						assert.Equal(t, float64(100), vars["first"])
					}),
				)
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.GraphQLQuery(searchResp(false, "", 101, minimalNode("D101", "Discussion 101")), func(_ string, vars map[string]interface{}) {
						assert.Equal(t, float64(1), vars["first"])
					}),
				)
			},
			wantLen:   101,
			wantTotal: 101,
		},
		{
			// When the page has more items than requested, NextCursor is set.
			name:  "pagination sets next cursor",
			limit: 1,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.StringResponse(searchResp(true, "searchCursor42", 5, minimalNode("D1", "Discussion 1"))),
				)
			},
			wantLen:    1,
			wantTotal:  5,
			wantCursor: "searchCursor42",
		},
		{
			// Two pages are fetched when limit exceeds the first page's results.
			name:  "pagination fetches multiple pages",
			limit: 2,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.StringResponse(searchResp(true, "searchCursor1", 2, minimalNode("D1", "First"))),
				)
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.StringResponse(searchResp(false, "", 2, minimalNode("D2", "Second"))),
				)
			},
			wantLen:    2,
			wantTotal:  2,
			wantTitles: []string{"First", "Second"},
		},
		{
			name:  "exact fit does not overfetch",
			limit: 1,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionListSearch\b`),
					httpmock.StringResponse(searchResp(false, "", 1, minimalNode("D1", "Only one"))),
				)
			},
			wantLen:    1,
			wantTotal:  1,
			wantTitles: []string{"Only one"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
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

			if tt.wantSingleDisc != nil {
				require.NotEmpty(t, result.Discussions)
				assert.Equal(t, *tt.wantSingleDisc, result.Discussions[0])
			}
		})
	}
}

func TestListCategories(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	tests := []struct {
		name      string
		httpStubs func(*testing.T, *httpmock.Registry)
		wantErr   string
		wantCats  []DiscussionCategory
	}{
		{
			name: "maps all fields",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionCategoryList\b`),
					httpmock.StringResponse(`{"data":{"repository":{
						"hasDiscussionsEnabled":true,
						"discussionCategories":{"nodes":[
							{"id":"C1","name":"General","slug":"general","emoji":":speech_balloon:","isAnswerable":false},
							{"id":"C2","name":"Q&A","slug":"q-a","emoji":":question:","isAnswerable":true}
						]}
					}}}`),
				)
			},
			wantCats: []DiscussionCategory{
				{ID: "C1", Name: "General", Slug: "general", Emoji: ":speech_balloon:", IsAnswerable: false},
				{ID: "C2", Name: "Q&A", Slug: "q-a", Emoji: ":question:", IsAnswerable: true},
			},
		},
		{
			name: "discussions disabled",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionCategoryList\b`),
					httpmock.StringResponse(`{"data":{"repository":{
						"hasDiscussionsEnabled":false,
						"discussionCategories":{"nodes":[]}
					}}}`),
				)
			},
			wantErr: "discussions disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
			}

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

func TestGetByNumber(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	tests := []struct {
		name       string
		httpStubs  func(*testing.T, *httpmock.Registry)
		wantErr    string
		assertDisc *Discussion
	}{
		{
			name: "maps all fields",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionMinimal\b`),
					httpmock.StringResponse(heredoc.Doc(`
						{
							"data": {
								"repository": {
									"hasDiscussionsEnabled": true,
									"discussion": {
										"id": "D_1",
										"number": 42,
										"title": "Test Discussion",
										"body": "This is a test",
										"url": "https://github.com/OWNER/REPO/discussions/42",
										"closed": true,
										"stateReason": "RESOLVED",
										"isAnswered": true,
										"answerChosenAt": "2025-06-01T12:00:00Z",
										"author": {"__typename": "User", "login": "alice", "id": "U1", "name": "Alice"},
										"category": {"id": "C1", "name": "Q&A", "slug": "q-a", "emoji": ":question:", "isAnswerable": true},
										"answerChosenBy": {"__typename": "User", "login": "bob", "id": "U2", "name": "Bob"},
										"labels": {"nodes": [{"id": "L1", "name": "bug", "color": "d73a4a"}]},
										"reactionGroups": [{"content": "THUMBS_UP", "users": {"totalCount": 3}}],
										"createdAt": "2025-01-01T00:00:00Z",
										"updatedAt": "2025-01-02T00:00:00Z",
										"closedAt": "2025-06-01T00:00:00Z",
										"locked": true,
										"comments": {"totalCount": 5}
									}
								}
							}
						}
					`)),
				)
			},
			assertDisc: &Discussion{
				ID:          "D_1",
				Number:      42,
				Title:       "Test Discussion",
				Body:        "This is a test",
				URL:         "https://github.com/OWNER/REPO/discussions/42",
				Closed:      true,
				StateReason: "RESOLVED",
				Author:      DiscussionActor{ID: "U1", Login: "alice", Name: "Alice"},
				Category: DiscussionCategory{
					ID:           "C1",
					Name:         "Q&A",
					Slug:         "q-a",
					Emoji:        ":question:",
					IsAnswerable: true,
				},
				Labels:         []DiscussionLabel{{ID: "L1", Name: "bug", Color: "d73a4a"}},
				Answered:       true,
				AnswerChosenAt: time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
				AnswerChosenBy: &DiscussionActor{ID: "U2", Login: "bob", Name: "Bob"},
				ReactionGroups: []ReactionGroup{
					{Content: "THUMBS_UP", TotalCount: 3},
				},
				Comments:  DiscussionCommentList{TotalCount: 5},
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
				ClosedAt:  time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
				Locked:    true,
			},
		},
		{
			name: "discussions disabled",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionMinimal\b`),
					httpmock.StringResponse(heredoc.Doc(`
						{
							"data": {
								"repository": {
									"hasDiscussionsEnabled": false,
									"discussion": null
								}
							},
							"errors": [
								{
									"type": "NOT_FOUND",
									"path": ["repository", "discussion"],
									"message": "Could not resolve to a Discussion with the number of 42."
								}
							]
						}
					`)),
				)
			},
			wantErr: "Could not resolve to a Discussion with the number of 42.",
		},
		{
			name: "repo not found",
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`query DiscussionMinimal\b`),
					httpmock.StringResponse(heredoc.Doc(`
						{
							"data": {
								"repository": null
							},
							"errors": [
								{
									"type": "NOT_FOUND",
									"path": ["repository"],
									"message": "Could not resolve to a Repository with the name 'OWNER/REPO'."
								}
							]
						}
					`)),
				)
			},
			wantErr: "Could not resolve to a Repository with the name 'OWNER/REPO'.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
			}

			c := newTestDiscussionClient(reg)
			d, err := c.GetByNumber(repo, 42)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, d)
			require.NotNil(t, tt.assertDisc, "assertDisc must be set for non-error cases")
			assert.Equal(t, tt.assertDisc, d)
		})
	}
}

// ---------------------------------------------------------------------------
// GetWithComments
// ---------------------------------------------------------------------------

// getWithCommentsResp builds a mock DiscussionWithComments JSON response.
func getWithCommentsResp(hasDiscussions bool, node string, commentNodes string, commentTotal int, hasNext, hasPrev bool, endCursor, startCursor string) string {
	return heredoc.Docf(`
		{
			"data": {
				"repository": {
					"hasDiscussionsEnabled": %t,
					"discussion": %s
				}
			}
		}
	`, hasDiscussions, wrapCommentsBlock(node, commentNodes, commentTotal, hasNext, hasPrev, endCursor, startCursor))
}

func wrapCommentsBlock(node string, commentNodes string, total int, hasNext, hasPrev bool, endCursor, startCursor string) string {
	trimmed := strings.TrimRight(node, " \t\n")
	trimmed = trimmed[:len(trimmed)-1]
	return fmt.Sprintf(`%s, "comments": {"totalCount": %d, "pageInfo": {"endCursor": %q, "hasNextPage": %t, "startCursor": %q, "hasPreviousPage": %t}, "nodes": [%s]}}`,
		trimmed, total, endCursor, hasNext, startCursor, hasPrev, commentNodes)
}

// commentNode builds a JSON comment node with nested replies.
func commentNode(id, login, body string, isAnswer bool, replyNodes string, replyTotal int) string {
	return heredoc.Docf(`
		{
			"id": %q,
			"url": "https://github.com/OWNER/REPO/discussions/1#comment-%s",
			"author": {"__typename": "User", "login": %q},
			"body": %q,
			"createdAt": "2025-01-01T00:00:00Z",
			"isAnswer": %t,
			"upvoteCount": 0,
			"reactionGroups": [],
			"replies": {"totalCount": %d, "nodes": [%s]}
		}
	`, id, id, login, body, isAnswer, replyTotal, replyNodes)
}

// replyNode builds a JSON reply node.
func replyNode(id, login, body string) string {
	return heredoc.Docf(`
		{
			"id": %q,
			"url": "https://github.com/OWNER/REPO/discussions/1#reply-%s",
			"author": {"__typename": "User", "login": %q},
			"body": %q,
			"createdAt": "2025-02-01T00:00:00Z",
			"isAnswer": false,
			"upvoteCount": 0,
			"reactionGroups": []
		}
	`, id, id, login, body)
}

func TestGetWithComments(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	tests := []struct {
		name          string
		limit         int
		after         string
		newest        bool
		httpStubs     func(*testing.T, *httpmock.Registry)
		wantErr       string
		wantComments  int
		wantTotal     int
		wantCursor    string
		wantNext      string
		wantDirection DiscussionCommentListDirection
		wantDisc      func(*testing.T, *Discussion)
	}{
		{
			name:   "maps comments with replies",
			limit:  10,
			newest: false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reply := replyNode("R1", "hubot", "Thanks!")
				comment := commentNode("C1", "octocat", "Main comment", true, reply, 1)
				node := minimalNode("D_1", "Test Discussion")
				reg.Register(
					httpmock.GraphQL(`query DiscussionWithComments\b`),
					httpmock.StringResponse(getWithCommentsResp(true, node, comment, 1, false, false, "", "")),
				)
			},
			wantComments:  1,
			wantTotal:     1,
			wantDirection: DiscussionCommentListDirectionForward,
			wantDisc: func(t *testing.T, d *Discussion) {
				c := d.Comments.Comments[0]
				assert.Equal(t, "C1", c.ID)
				assert.Equal(t, "octocat", c.Author.Login)
				assert.True(t, c.IsAnswer)
				require.Len(t, c.Replies.Comments, 1)
				assert.Equal(t, "R1", c.Replies.Comments[0].ID)
				assert.Equal(t, "hubot", c.Replies.Comments[0].Author.Login)
			},
		},
		{
			name:   "pagination forward",
			limit:  5,
			after:  "CUR_A",
			newest: false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				comment := commentNode("C1", "alice", "Hello", false, "", 0)
				node := minimalNode("D_1", "Test")
				reg.Register(
					httpmock.GraphQL(`query DiscussionWithComments\b`),
					httpmock.StringResponse(getWithCommentsResp(true, node, comment, 3, true, false, "CUR_B", "")),
				)
			},
			wantComments:  1,
			wantTotal:     3,
			wantCursor:    "CUR_A",
			wantNext:      "CUR_B",
			wantDirection: DiscussionCommentListDirectionForward,
		},
		{
			name:   "pagination backward newest",
			limit:  5,
			after:  "CUR_X",
			newest: true,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				c1 := commentNode("C1", "alice", "First", false, "", 0)
				c2 := commentNode("C2", "bob", "Second", false, "", 0)
				node := minimalNode("D_1", "Test")
				reg.Register(
					httpmock.GraphQL(`query DiscussionWithComments\b`),
					httpmock.StringResponse(getWithCommentsResp(true, node, c1+","+c2, 5, false, true, "", "CUR_Y")),
				)
			},
			wantComments:  2,
			wantTotal:     5,
			wantCursor:    "CUR_X",
			wantNext:      "CUR_Y",
			wantDirection: DiscussionCommentListDirectionBackward,
			wantDisc: func(t *testing.T, d *Discussion) {
				// Newest mode reverses the order
				assert.Equal(t, "C2", d.Comments.Comments[0].ID)
				assert.Equal(t, "C1", d.Comments.Comments[1].ID)
			},
		},
		{
			name:   "no more pages",
			limit:  10,
			newest: false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				comment := commentNode("C1", "alice", "Only one", false, "", 0)
				node := minimalNode("D_1", "Test")
				reg.Register(
					httpmock.GraphQL(`query DiscussionWithComments\b`),
					httpmock.StringResponse(getWithCommentsResp(true, node, comment, 1, false, false, "", "")),
				)
			},
			wantComments:  1,
			wantTotal:     1,
			wantNext:      "",
			wantDirection: DiscussionCommentListDirectionForward,
		},
		{
			name:   "discussions disabled",
			limit:  10,
			newest: false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				node := minimalNode("D_1", "Test")
				reg.Register(
					httpmock.GraphQL(`query DiscussionWithComments\b`),
					httpmock.StringResponse(getWithCommentsResp(false, node, "", 0, false, false, "", "")),
				)
			},
			wantErr: "discussions disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
			}

			c := newTestDiscussionClient(reg)
			d, err := c.GetWithComments(repo, 1, tt.limit, tt.after, tt.newest)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, d)
			assert.Len(t, d.Comments.Comments, tt.wantComments)
			assert.Equal(t, tt.wantTotal, d.Comments.TotalCount)
			assert.Equal(t, tt.wantCursor, d.Comments.Cursor)
			assert.Equal(t, tt.wantNext, d.Comments.NextCursor)
			assert.Equal(t, tt.wantDirection, d.Comments.Direction)
			if tt.wantDisc != nil {
				tt.wantDisc(t, d)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetCommentReplies
// ---------------------------------------------------------------------------

// getCommentRepliesResp builds a mock DiscussionCommentReplies JSON response.
// The shurcooL graphql library treats inline fragments as transparent — the
// comment fields are placed directly inside the "node" object, not nested
// under a "DiscussionComment" key.
func getCommentRepliesResp(hasDiscussions bool, discNode string, commentNode *string, replyNodes string, replyTotal int, hasNext, hasPrev bool, endCursor, startCursor string) string {
	nodeBlock := "null"
	if commentNode != nil {
		nodeBlock = wrapRepliesBlock(*commentNode, replyNodes, replyTotal, hasNext, hasPrev, endCursor, startCursor)
	}
	return heredoc.Docf(`
		{
			"data": {
				"repository": {
					"hasDiscussionsEnabled": %t,
					"discussion": %s
				},
				"node": %s
			}
		}
	`, hasDiscussions, discNode, nodeBlock)
}

func wrapRepliesBlock(commentJSON string, replyNodes string, total int, hasNext, hasPrev bool, endCursor, startCursor string) string {
	trimmed := strings.TrimRight(commentJSON, " \t\n")
	trimmed = trimmed[:len(trimmed)-1]
	return fmt.Sprintf(`%s, "replies": {"totalCount": %d, "pageInfo": {"endCursor": %q, "hasNextPage": %t, "startCursor": %q, "hasPreviousPage": %t}, "nodes": [%s]}}`,
		trimmed, total, endCursor, hasNext, startCursor, hasPrev, replyNodes)
}

// bareCommentNode builds a comment JSON node without replies (used for GetCommentReplies).
func bareCommentNode(id, login, body string, isAnswer bool) string {
	return heredoc.Docf(`
		{
			"id": %q,
			"url": "https://github.com/OWNER/REPO/discussions/1#comment-%s",
			"author": {"__typename": "User", "login": %q},
			"body": %q,
			"createdAt": "2025-01-01T00:00:00Z",
			"isAnswer": %t,
			"upvoteCount": 2,
			"reactionGroups": [{"content": "HEART", "users": {"totalCount": 1}}]
		}
	`, id, id, login, body, isAnswer)
}

func TestGetCommentReplies(t *testing.T) {
	repo := ghrepo.New("OWNER", "REPO")

	tests := []struct {
		name          string
		commentID     string
		limit         int
		after         string
		newest        bool
		httpStubs     func(*testing.T, *httpmock.Registry)
		wantErr       string
		wantReplies   int
		wantTotal     int
		wantCursor    string
		wantNext      string
		wantDirection DiscussionCommentListDirection
		wantDisc      func(*testing.T, *Discussion)
	}{
		{
			name:      "maps all fields",
			commentID: "DC_abc",
			limit:     10,
			newest:    false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test Discussion")
				comment := bareCommentNode("DC_abc", "octocat", "Top-level comment", true)
				reply := replyNode("R1", "hubot", "A reply")
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(true, discNode, &comment, reply, 1, false, false, "", "")),
				)
			},
			wantReplies:   1,
			wantTotal:     1,
			wantDirection: DiscussionCommentListDirectionForward,
			wantDisc: func(t *testing.T, d *Discussion) {
				assert.Equal(t, "Test Discussion", d.Title)
				require.Len(t, d.Comments.Comments, 1)
				c := d.Comments.Comments[0]
				assert.Equal(t, "DC_abc", c.ID)
				assert.Equal(t, "octocat", c.Author.Login)
				assert.Equal(t, "Top-level comment", c.Body)
				assert.True(t, c.IsAnswer)
				assert.Equal(t, 2, c.UpvoteCount)
				require.Len(t, c.ReactionGroups, 1)
				assert.Equal(t, "HEART", c.ReactionGroups[0].Content)
				require.Len(t, c.Replies.Comments, 1)
				assert.Equal(t, "R1", c.Replies.Comments[0].ID)
				assert.Equal(t, "hubot", c.Replies.Comments[0].Author.Login)
			},
		},
		{
			name:      "pagination forward oldest",
			commentID: "DC_abc",
			limit:     5,
			after:     "CUR_A",
			newest:    false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test")
				comment := bareCommentNode("DC_abc", "alice", "Comment", false)
				r1 := replyNode("R1", "bob", "Reply 1")
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(true, discNode, &comment, r1, 3, true, false, "CUR_B", "")),
				)
			},
			wantReplies:   1,
			wantTotal:     3,
			wantCursor:    "CUR_A",
			wantNext:      "CUR_B",
			wantDirection: DiscussionCommentListDirectionForward,
		},
		{
			name:      "pagination backward newest reverses replies",
			commentID: "DC_abc",
			limit:     5,
			after:     "CUR_X",
			newest:    true,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test")
				comment := bareCommentNode("DC_abc", "alice", "Comment", false)
				r1 := replyNode("R1", "bob", "Older")
				r2 := replyNode("R2", "carol", "Newer")
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(true, discNode, &comment, r1+","+r2, 5, false, true, "", "CUR_Y")),
				)
			},
			wantReplies:   2,
			wantTotal:     5,
			wantCursor:    "CUR_X",
			wantNext:      "CUR_Y",
			wantDirection: DiscussionCommentListDirectionBackward,
			wantDisc: func(t *testing.T, d *Discussion) {
				replies := d.Comments.Comments[0].Replies.Comments
				assert.Equal(t, "R2", replies[0].ID, "newest mode should reverse replies")
				assert.Equal(t, "R1", replies[1].ID)
			},
		},
		{
			name:      "no more pages",
			commentID: "DC_abc",
			limit:     10,
			newest:    false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test")
				comment := bareCommentNode("DC_abc", "alice", "Comment", false)
				r1 := replyNode("R1", "bob", "Only reply")
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(true, discNode, &comment, r1, 1, false, false, "", "")),
				)
			},
			wantReplies:   1,
			wantTotal:     1,
			wantNext:      "",
			wantDirection: DiscussionCommentListDirectionForward,
		},
		{
			name:      "discussions disabled",
			commentID: "DC_abc",
			limit:     10,
			newest:    false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test")
				comment := bareCommentNode("DC_abc", "alice", "Comment", false)
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(false, discNode, &comment, "", 0, false, false, "", "")),
				)
			},
			wantErr: "discussions disabled",
		},
		{
			name:      "node not found nil",
			commentID: "DC_invalid",
			limit:     10,
			newest:    false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test")
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(true, discNode, nil, "", 0, false, false, "", "")),
				)
			},
			wantErr: "comment DC_invalid not found",
		},
		{
			name:      "node is not a discussion comment",
			commentID: "I_notacomment",
			limit:     10,
			newest:    false,
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				discNode := minimalNode("D_1", "Test")
				// Return a node with an empty DiscussionComment (wrong type)
				emptyComment := `{"id":"","url":"","author":{"__typename":"User","login":""},"body":"","createdAt":"0001-01-01T00:00:00Z","isAnswer":false,"upvoteCount":0,"reactionGroups":[]}`
				reg.Register(
					httpmock.GraphQL(`query DiscussionCommentReplies\b`),
					httpmock.StringResponse(getCommentRepliesResp(true, discNode, &emptyComment, "", 0, false, false, "", "")),
				)
			},
			wantErr: "node I_notacomment is not a discussion comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
			}

			c := newTestDiscussionClient(reg)
			d, err := c.GetCommentReplies(repo, 1, tt.commentID, tt.limit, tt.after, tt.newest)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, d)
			require.Len(t, d.Comments.Comments, 1, "GetCommentReplies should return exactly one comment")

			comment := d.Comments.Comments[0]
			assert.Len(t, comment.Replies.Comments, tt.wantReplies)
			assert.Equal(t, tt.wantTotal, comment.Replies.TotalCount)
			assert.Equal(t, tt.wantCursor, comment.Replies.Cursor)
			assert.Equal(t, tt.wantNext, comment.Replies.NextCursor)
			assert.Equal(t, tt.wantDirection, comment.Replies.Direction)
			if tt.wantDisc != nil {
				tt.wantDisc(t, d)
			}
		})
	}
}
