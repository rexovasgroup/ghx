package client

import (
	"net/http"
	"testing"

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

func ptr[T any](v T) *T { return &v }

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_success(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	reg.Register(
		httpmock.GraphQL(`query DiscussionList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": true,
			"discussions": {
				"totalCount": 1,
				"pageInfo": {"hasNextPage": false, "endCursor": ""},
				"nodes": [{
					"id": "D_id1",
					"number": 1,
					"title": "Hello world",
					"body": "body text",
					"url": "https://github.com/OWNER/REPO/discussions/1",
					"closed": false,
					"stateReason": "",
					"isAnswered": false,
					"answerChosenAt": "0001-01-01T00:00:00Z",
					"author": {"__typename": "User", "login": "alice", "id": "U1", "name": "Alice"},
					"category": {"id": "C1", "name": "General", "slug": "general", "emoji": ":speech_balloon:", "isAnswerable": false},
					"answerChosenBy": null,
					"labels": {"nodes": []},
					"reactionGroups": [],
					"createdAt": "2024-01-01T00:00:00Z",
					"updatedAt": "2024-01-02T00:00:00Z",
					"closedAt": "0001-01-01T00:00:00Z",
					"locked": false
				}]
			}
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	result, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{}, "", 10)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Discussions, 1)
	assert.Equal(t, "Hello world", result.Discussions[0].Title)
	assert.Equal(t, "", result.NextCursor)
}

func TestList_discussionsDisabled(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	reg.Register(
		httpmock.GraphQL(`query DiscussionList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": false,
			"discussions": {"totalCount": 0, "pageInfo": {"hasNextPage": false, "endCursor": ""}, "nodes": []}
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	_, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discussions disabled")
}

func TestList_limitZero(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{}, "", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit argument must be positive")
}

func TestList_invalidOrderBy(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{OrderBy: "invalid"}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown order-by field")
}

func TestList_invalidDirection(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{Direction: "sideways"}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown order direction")
}

func TestList_invalidState(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{State: ptr("merged")}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown state filter")
}

func TestList_pagination(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	// First page
	reg.Register(
		httpmock.GraphQL(`query DiscussionList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": true,
			"discussions": {
				"totalCount": 2,
				"pageInfo": {"hasNextPage": true, "endCursor": "cursor1"},
				"nodes": [{
					"id": "D1", "number": 1, "title": "Discussion 1", "body": "",
					"url": "", "closed": false, "stateReason": "", "isAnswered": false,
					"answerChosenAt": "0001-01-01T00:00:00Z",
					"author": {"__typename": "User", "login": "alice"},
					"category": {"id": "C1", "name": "General", "slug": "general", "emoji": "", "isAnswerable": false},
					"answerChosenBy": null, "labels": {"nodes": []}, "reactionGroups": [],
					"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z",
					"closedAt": "0001-01-01T00:00:00Z", "locked": false
				}]
			}
		}}}`),
	)
	// Second page
	reg.Register(
		httpmock.GraphQL(`query DiscussionList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": true,
			"discussions": {
				"totalCount": 2,
				"pageInfo": {"hasNextPage": false, "endCursor": ""},
				"nodes": [{
					"id": "D2", "number": 2, "title": "Discussion 2", "body": "",
					"url": "", "closed": false, "stateReason": "", "isAnswered": false,
					"answerChosenAt": "0001-01-01T00:00:00Z",
					"author": {"__typename": "User", "login": "bob"},
					"category": {"id": "C1", "name": "General", "slug": "general", "emoji": "", "isAnswerable": false},
					"answerChosenBy": null, "labels": {"nodes": []}, "reactionGroups": [],
					"createdAt": "2024-01-02T00:00:00Z", "updatedAt": "2024-01-02T00:00:00Z",
					"closedAt": "0001-01-01T00:00:00Z", "locked": false
				}]
			}
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	// limit > 1 forces pagination across both pages
	result, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{}, "", 2)
	require.NoError(t, err)
	assert.Len(t, result.Discussions, 2)
	assert.Equal(t, "Discussion 1", result.Discussions[0].Title)
	assert.Equal(t, "Discussion 2", result.Discussions[1].Title)
	assert.Equal(t, "", result.NextCursor)
}

func TestList_paginationSetsNextCursor(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	// When the caller requests fewer items than are available, NextCursor should be set.
	reg.Register(
		httpmock.GraphQL(`query DiscussionList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": true,
			"discussions": {
				"totalCount": 5,
				"pageInfo": {"hasNextPage": true, "endCursor": "cursor42"},
				"nodes": [{
					"id": "D1", "number": 1, "title": "Discussion 1", "body": "",
					"url": "", "closed": false, "stateReason": "", "isAnswered": false,
					"answerChosenAt": "0001-01-01T00:00:00Z",
					"author": {"__typename": "User", "login": "alice"},
					"category": {"id": "C1", "name": "General", "slug": "general", "emoji": "", "isAnswerable": false},
					"answerChosenBy": null, "labels": {"nodes": []}, "reactionGroups": [],
					"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z",
					"closedAt": "0001-01-01T00:00:00Z", "locked": false
				}]
			}
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	result, err := c.List(ghrepo.New("OWNER", "REPO"), ListFilters{}, "", 1)
	require.NoError(t, err)
	assert.Len(t, result.Discussions, 1)
	assert.Equal(t, "cursor42", result.NextCursor)
}

func TestList_filters(t *testing.T) {
	tests := []struct {
		name    string
		filters ListFilters
	}{
		{
			name:    "open state",
			filters: ListFilters{State: ptr(FilterStateOpen)},
		},
		{
			name:    "closed state",
			filters: ListFilters{State: ptr(FilterStateClosed)},
		},
		{
			name:    "answered",
			filters: ListFilters{Answered: ptr(true)},
		},
		{
			name:    "unanswered",
			filters: ListFilters{Answered: ptr(false)},
		},
		{
			name:    "category ID",
			filters: ListFilters{CategoryID: "CAT123"},
		},
		{
			name:    "order by created asc",
			filters: ListFilters{OrderBy: OrderByCreated, Direction: OrderDirectionAsc},
		},
		{
			name:    "order by updated desc",
			filters: ListFilters{OrderBy: OrderByUpdated, Direction: OrderDirectionDesc},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			reg.Register(
				httpmock.GraphQL(`query DiscussionList\b`),
				httpmock.StringResponse(`{"data":{"repository":{
					"hasDiscussionsEnabled": true,
					"discussions": {"totalCount": 0, "pageInfo": {"hasNextPage": false, "endCursor": ""}, "nodes": []}
				}}}`),
			)

			c := newTestDiscussionClient(reg)
			result, err := c.List(ghrepo.New("OWNER", "REPO"), tt.filters, "", 10)
			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_success(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	reg.Register(
		httpmock.GraphQL(`query DiscussionListSearch\b`),
		httpmock.StringResponse(`{"data":{"search":{
			"discussionCount": 1,
			"pageInfo": {"hasNextPage": false, "endCursor": ""},
			"nodes": [{
				"id": "D1", "number": 1, "title": "Searched discussion", "body": "",
				"url": "", "closed": false, "stateReason": "", "isAnswered": false,
				"answerChosenAt": "0001-01-01T00:00:00Z",
				"author": {"__typename": "User", "login": "alice"},
				"category": {"id": "C1", "name": "General", "slug": "general", "emoji": "", "isAnswerable": false},
				"answerChosenBy": null, "labels": {"nodes": []}, "reactionGroups": [],
				"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z",
				"closedAt": "0001-01-01T00:00:00Z", "locked": false
			}]
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	result, err := c.Search(ghrepo.New("OWNER", "REPO"), SearchFilters{}, "", 10)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Discussions, 1)
	assert.Equal(t, "Searched discussion", result.Discussions[0].Title)
}

func TestSearch_limitZero(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.Search(ghrepo.New("OWNER", "REPO"), SearchFilters{}, "", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit argument must be positive")
}

func TestSearch_invalidOrderBy(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.Search(ghrepo.New("OWNER", "REPO"), SearchFilters{OrderBy: "bogus"}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown order-by field")
}

func TestSearch_invalidDirection(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.Search(ghrepo.New("OWNER", "REPO"), SearchFilters{Direction: "sideways"}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown order direction")
}

func TestSearch_invalidState(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	c := newTestDiscussionClient(reg)
	_, err := c.Search(ghrepo.New("OWNER", "REPO"), SearchFilters{State: ptr("merged")}, "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown state filter")
}

func TestSearch_filters(t *testing.T) {
	tests := []struct {
		name    string
		filters SearchFilters
	}{
		{
			name:    "open state",
			filters: SearchFilters{State: ptr(FilterStateOpen)},
		},
		{
			name:    "closed state",
			filters: SearchFilters{State: ptr(FilterStateClosed)},
		},
		{
			name:    "answered",
			filters: SearchFilters{Answered: ptr(true)},
		},
		{
			name:    "unanswered",
			filters: SearchFilters{Answered: ptr(false)},
		},
		{
			name:    "author",
			filters: SearchFilters{Author: "alice"},
		},
		{
			name:    "labels",
			filters: SearchFilters{Labels: []string{"bug", "enhancement"}},
		},
		{
			name:    "category",
			filters: SearchFilters{Category: "Q&A"},
		},
		{
			name:    "keywords",
			filters: SearchFilters{Keywords: "some keyword"},
		},
		{
			name:    "order by created asc",
			filters: SearchFilters{OrderBy: OrderByCreated, Direction: OrderDirectionAsc},
		},
		{
			name:    "order by updated desc",
			filters: SearchFilters{OrderBy: OrderByUpdated, Direction: OrderDirectionDesc},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			defer reg.Verify(t)

			reg.Register(
				httpmock.GraphQL(`query DiscussionListSearch\b`),
				httpmock.StringResponse(`{"data":{"search":{
					"discussionCount": 0,
					"pageInfo": {"hasNextPage": false, "endCursor": ""},
					"nodes": []
				}}}`),
			)

			c := newTestDiscussionClient(reg)
			result, err := c.Search(ghrepo.New("OWNER", "REPO"), tt.filters, "", 10)
			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func TestSearch_pagination(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	makeNode := func(id, title string) string {
		return `{
			"id": "` + id + `", "number": 1, "title": "` + title + `", "body": "",
			"url": "", "closed": false, "stateReason": "", "isAnswered": false,
			"answerChosenAt": "0001-01-01T00:00:00Z",
			"author": {"__typename": "User", "login": "alice"},
			"category": {"id": "C1", "name": "General", "slug": "general", "emoji": "", "isAnswerable": false},
			"answerChosenBy": null, "labels": {"nodes": []}, "reactionGroups": [],
			"createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z",
			"closedAt": "0001-01-01T00:00:00Z", "locked": false
		}`
	}

	reg.Register(
		httpmock.GraphQL(`query DiscussionListSearch\b`),
		httpmock.StringResponse(`{"data":{"search":{
			"discussionCount": 2,
			"pageInfo": {"hasNextPage": true, "endCursor": "searchCursor1"},
			"nodes": [`+makeNode("D1", "First")+`]
		}}}`),
	)
	reg.Register(
		httpmock.GraphQL(`query DiscussionListSearch\b`),
		httpmock.StringResponse(`{"data":{"search":{
			"discussionCount": 2,
			"pageInfo": {"hasNextPage": false, "endCursor": ""},
			"nodes": [`+makeNode("D2", "Second")+`]
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	result, err := c.Search(ghrepo.New("OWNER", "REPO"), SearchFilters{}, "", 2)
	require.NoError(t, err)
	assert.Len(t, result.Discussions, 2)
	assert.Equal(t, "First", result.Discussions[0].Title)
	assert.Equal(t, "Second", result.Discussions[1].Title)
}

// ---------------------------------------------------------------------------
// ListCategories
// ---------------------------------------------------------------------------

func TestListCategories_success(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	reg.Register(
		httpmock.GraphQL(`query DiscussionCategoryList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": true,
			"discussionCategories": {"nodes": [
				{"id": "C1", "name": "General", "slug": "general", "emoji": ":speech_balloon:", "isAnswerable": false},
				{"id": "C2", "name": "Q&A", "slug": "q-a", "emoji": ":question:", "isAnswerable": true}
			]}
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	categories, err := c.ListCategories(ghrepo.New("OWNER", "REPO"))
	require.NoError(t, err)
	require.Len(t, categories, 2)
	assert.Equal(t, "General", categories[0].Name)
	assert.Equal(t, "Q&A", categories[1].Name)
	assert.True(t, categories[1].IsAnswerable)
}

func TestListCategories_discussionsDisabled(t *testing.T) {
	reg := &httpmock.Registry{}
	defer reg.Verify(t)

	reg.Register(
		httpmock.GraphQL(`query DiscussionCategoryList\b`),
		httpmock.StringResponse(`{"data":{"repository":{
			"hasDiscussionsEnabled": false,
			"discussionCategories": {"nodes": []}
		}}}`),
	)

	c := newTestDiscussionClient(reg)
	_, err := c.ListCategories(ghrepo.New("OWNER", "REPO"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discussions disabled")
}
