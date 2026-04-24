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
							"endCursor": ""
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
