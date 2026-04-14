package client

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/shurcooL/githubv4"
)

type discussionClient struct {
	gql *api.Client
}

// NewDiscussionClient creates a DiscussionClient backed by the given HTTP client.
func NewDiscussionClient(httpClient *http.Client) DiscussionClient {
	return &discussionClient{
		gql: api.NewClientFromHTTP(httpClient),
	}
}

// discussionNode is the shared GraphQL response shape for a single discussion,
// used by both List and Search to avoid duplicating the field mapping.
type discussionNode struct {
	ID          string `json:"id"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Closed      bool   `json:"closed"`
	StateReason string `json:"stateReason"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
	Category struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		Emoji        string `json:"emoji"`
		IsAnswerable bool   `json:"isAnswerable"`
	} `json:"category"`
	Labels struct {
		Nodes []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"nodes"`
	} `json:"labels"`
	IsAnswered     bool      `json:"isAnswered"`
	AnswerChosenAt time.Time `json:"answerChosenAt"`
	AnswerChosenBy *struct {
		Login string `json:"login"`
	} `json:"answerChosenBy"`
	ReactionGroups []struct {
		Content string `json:"content"`
		Users   struct {
			TotalCount int `json:"totalCount"`
		} `json:"users"`
	} `json:"reactionGroups"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	ClosedAt  time.Time `json:"closedAt"`
	Locked    bool      `json:"locked"`
}

// mapDiscussion converts a GraphQL discussionNode response into the domain Discussion type.
func mapDiscussion(n discussionNode) Discussion {
	d := Discussion{
		ID:          n.ID,
		Number:      n.Number,
		Title:       n.Title,
		URL:         n.URL,
		Closed:      n.Closed,
		StateReason: n.StateReason,
		Author:      DiscussionAuthor{Login: n.Author.Login},
		Category: DiscussionCategory{
			ID:           n.Category.ID,
			Name:         n.Category.Name,
			Slug:         n.Category.Slug,
			Emoji:        n.Category.Emoji,
			IsAnswerable: n.Category.IsAnswerable,
		},
		Answered:       n.IsAnswered,
		AnswerChosenAt: n.AnswerChosenAt,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
		ClosedAt:       n.ClosedAt,
		Locked:         n.Locked,
	}

	if n.AnswerChosenBy != nil {
		d.AnswerChosenBy = &DiscussionAuthor{Login: n.AnswerChosenBy.Login}
	}

	d.Labels = make([]DiscussionLabel, len(n.Labels.Nodes))
	for i, l := range n.Labels.Nodes {
		d.Labels[i] = DiscussionLabel{ID: l.ID, Name: l.Name, Color: l.Color}
	}

	d.ReactionGroups = make([]ReactionGroup, len(n.ReactionGroups))
	for i, rg := range n.ReactionGroups {
		d.ReactionGroups[i] = ReactionGroup{Content: rg.Content, TotalCount: rg.Users.TotalCount}
	}

	return d
}

// discussionFields is the GraphQL fragment selecting fields for discussion queries.
// It is shared by both List (repository.discussions) and Search queries.
const discussionFields = `
	id number title url closed stateReason
	author { login }
	category { id name slug emoji isAnswerable }
	labels(first: 20) { nodes { id name color } }
	isAnswered answerChosenAt answerChosenBy { login }
	reactionGroups { content users { totalCount } }
	createdAt updatedAt closedAt locked
`

func (c *discussionClient) List(repo ghrepo.Interface, filters ListFilters, after string, limit int) (DiscussionListResult, error) {
	if limit <= 0 {
		return DiscussionListResult{}, fmt.Errorf("limit argument must be positive: %v", limit)
	}

	type response struct {
		Repository struct {
			HasDiscussionsEnabled bool `json:"hasDiscussionsEnabled"`
			Discussions           struct {
				TotalCount int `json:"totalCount"`
				PageInfo   struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []discussionNode `json:"nodes"`
			} `json:"discussions"`
		} `json:"repository"`
	}

	variables := map[string]interface{}{
		"owner": repo.RepoOwner(),
		"name":  repo.RepoName(),
	}

	orderField := "UPDATED_AT"
	orderDir := "DESC"
	if filters.OrderBy != "" {
		switch filters.OrderBy {
		case OrderByCreated:
			orderField = "CREATED_AT"
		case OrderByUpdated:
			orderField = "UPDATED_AT"
		default:
			return DiscussionListResult{}, fmt.Errorf("unknown order-by field: %q", filters.OrderBy)
		}
	}
	if filters.Direction != "" {
		switch filters.Direction {
		case OrderDirectionAsc:
			orderDir = "ASC"
		case OrderDirectionDesc:
			orderDir = "DESC"
		default:
			return DiscussionListResult{}, fmt.Errorf("unknown order direction: %q", filters.Direction)
		}
	}
	variables["orderBy"] = map[string]string{
		"field":     orderField,
		"direction": orderDir,
	}

	if filters.CategoryID != "" {
		variables["categoryId"] = filters.CategoryID
	}

	if filters.State != nil {
		switch *filters.State {
		case FilterStateOpen:
			variables["states"] = []string{"OPEN"}
		case FilterStateClosed:
			variables["states"] = []string{"CLOSED"}
		default:
			return DiscussionListResult{}, fmt.Errorf("unknown state filter: %q; should be one of %q, %q", *filters.State, FilterStateOpen, FilterStateClosed)
		}
	}

	if filters.Answered != nil {
		variables["answered"] = *filters.Answered
	}

	// Build optional parameter declarations
	paramParts := []string{
		"$owner: String!",
		"$name: String!",
		"$first: Int!",
		"$after: String",
		"$orderBy: DiscussionOrder",
	}
	argParts := []string{
		"first: $first",
		"after: $after",
		"orderBy: $orderBy",
	}
	if filters.CategoryID != "" {
		paramParts = append(paramParts, "$categoryId: ID")
		argParts = append(argParts, "categoryId: $categoryId")
	}
	if _, ok := variables["states"]; ok {
		paramParts = append(paramParts, "$states: [DiscussionState!]")
		argParts = append(argParts, "states: $states")
	}
	if filters.Answered != nil {
		paramParts = append(paramParts, "$answered: Boolean")
		argParts = append(argParts, "answered: $answered")
	}

	query := fmt.Sprintf(`query DiscussionList(%s) {
		repository(owner: $owner, name: $name) {
			hasDiscussionsEnabled
			discussions(%s) {
				totalCount
				pageInfo { hasNextPage endCursor }
				nodes { %s }
			}
		}
	}`, strings.Join(paramParts, ", "), strings.Join(argParts, ", "), discussionFields)

	if after != "" {
		variables["after"] = after
	}

	var discussions []Discussion
	var totalCount int
	var nextCursor string
	remaining := limit

	// Check hasDiscussionsEnabled on first request only
	firstPage := true

	for {
		perPage := remaining
		if perPage > 100 {
			perPage = 100
		}
		variables["first"] = perPage

		var data response
		if err := c.gql.GraphQL(repo.RepoHost(), query, variables, &data); err != nil {
			return DiscussionListResult{}, err
		}

		if firstPage && !data.Repository.HasDiscussionsEnabled {
			return DiscussionListResult{}, fmt.Errorf("the '%s/%s' repository has discussions disabled", repo.RepoOwner(), repo.RepoName())
		}
		firstPage = false

		totalCount = data.Repository.Discussions.TotalCount
		for _, n := range data.Repository.Discussions.Nodes {
			discussions = append(discussions, mapDiscussion(n))
		}

		remaining -= len(data.Repository.Discussions.Nodes)
		if remaining <= 0 || !data.Repository.Discussions.PageInfo.HasNextPage {
			if data.Repository.Discussions.PageInfo.HasNextPage {
				nextCursor = data.Repository.Discussions.PageInfo.EndCursor
			}
			break
		}
		variables["after"] = data.Repository.Discussions.PageInfo.EndCursor
	}

	return DiscussionListResult{
		Discussions: discussions,
		TotalCount:  totalCount,
		NextCursor:  nextCursor,
	}, nil
}

func (c *discussionClient) Search(repo ghrepo.Interface, filters SearchFilters, after string, limit int) (DiscussionListResult, error) {
	if limit <= 0 {
		return DiscussionListResult{}, fmt.Errorf("limit argument must be positive: %v", limit)
	}

	type response struct {
		Search struct {
			DiscussionCount int `json:"discussionCount"`
			PageInfo        struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []discussionNode `json:"nodes"`
		} `json:"search"`
	}

	qualifiers := []string{fmt.Sprintf("repo:%s/%s", repo.RepoOwner(), repo.RepoName())}

	if filters.State != nil {
		switch *filters.State {
		case FilterStateOpen:
			qualifiers = append(qualifiers, "state:open")
		case FilterStateClosed:
			qualifiers = append(qualifiers, "state:closed")
		default:
			return DiscussionListResult{}, fmt.Errorf("unknown state filter: %q; should be one of %q, %q", *filters.State, FilterStateOpen, FilterStateClosed)
		}
	}

	if filters.Author != "" {
		qualifiers = append(qualifiers, fmt.Sprintf("author:%q", filters.Author))
	}
	for _, l := range filters.Labels {
		qualifiers = append(qualifiers, fmt.Sprintf("label:%q", l))
	}
	if filters.Category != "" {
		qualifiers = append(qualifiers, fmt.Sprintf("category:%q", filters.Category))
	}
	if filters.Answered != nil {
		if *filters.Answered {
			qualifiers = append(qualifiers, "is:answered")
		} else {
			qualifiers = append(qualifiers, "is:unanswered")
		}
	}

	orderField := OrderByUpdated
	orderDir := OrderDirectionDesc
	if filters.OrderBy != "" {
		switch filters.OrderBy {
		case OrderByCreated, OrderByUpdated:
			orderField = filters.OrderBy
		default:
			return DiscussionListResult{}, fmt.Errorf("unknown order-by field: %q", filters.OrderBy)
		}
	}
	if filters.Direction != "" {
		switch filters.Direction {
		case OrderDirectionAsc, OrderDirectionDesc:
			orderDir = filters.Direction
		default:
			return DiscussionListResult{}, fmt.Errorf("unknown order direction: %q", filters.Direction)
		}
	}
	qualifiers = append(qualifiers, fmt.Sprintf("sort:%s-%s", orderField, orderDir))

	searchQuery := strings.Join(qualifiers, " ")
	if filters.Keywords != "" {
		searchQuery += " " + filters.Keywords
	}

	query := fmt.Sprintf(`query DiscussionSearch($query: String!, $first: Int!, $after: String) {
		search(query: $query, type: DISCUSSION, first: $first, after: $after) {
			discussionCount
			pageInfo { hasNextPage endCursor }
			nodes { ... on Discussion { %s } }
		}
	}`, discussionFields)

	variables := map[string]interface{}{
		"query": searchQuery,
	}

	if after != "" {
		variables["after"] = after
	}

	var discussions []Discussion
	var totalCount int
	var nextCursor string
	remaining := limit

	for {
		perPage := remaining
		if perPage > 100 {
			perPage = 100
		}
		variables["first"] = perPage

		var data response
		if err := c.gql.GraphQL(repo.RepoHost(), query, variables, &data); err != nil {
			return DiscussionListResult{}, err
		}

		totalCount = data.Search.DiscussionCount
		for _, n := range data.Search.Nodes {
			discussions = append(discussions, mapDiscussion(n))
		}

		remaining -= len(data.Search.Nodes)
		if remaining <= 0 || !data.Search.PageInfo.HasNextPage {
			if data.Search.PageInfo.HasNextPage {
				nextCursor = data.Search.PageInfo.EndCursor
			}
			break
		}
		variables["after"] = data.Search.PageInfo.EndCursor
	}

	return DiscussionListResult{
		Discussions: discussions,
		TotalCount:  totalCount,
		NextCursor:  nextCursor,
	}, nil
}

func (c *discussionClient) GetByNumber(_ ghrepo.Interface, _ int) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) GetWithComments(_ ghrepo.Interface, _ int, _ int, _ string) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) ListCategories(repo ghrepo.Interface) ([]DiscussionCategory, error) {
	var query struct {
		Repository struct {
			HasDiscussionsEnabled bool
			DiscussionCategories  struct {
				Nodes []struct {
					ID           string
					Name         string
					Slug         string
					Emoji        string
					IsAnswerable bool
				}
			} `graphql:"discussionCategories(first: 100)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(repo.RepoOwner()),
		"name":  githubv4.String(repo.RepoName()),
	}

	if err := c.gql.Query(repo.RepoHost(), "DiscussionCategoryList", &query, variables); err != nil {
		return nil, err
	}

	if !query.Repository.HasDiscussionsEnabled {
		return nil, fmt.Errorf("the '%s/%s' repository has discussions disabled", repo.RepoOwner(), repo.RepoName())
	}

	categories := make([]DiscussionCategory, len(query.Repository.DiscussionCategories.Nodes))
	for i, n := range query.Repository.DiscussionCategories.Nodes {
		categories[i] = DiscussionCategory{
			ID:           n.ID,
			Name:         n.Name,
			Slug:         n.Slug,
			Emoji:        n.Emoji,
			IsAnswerable: n.IsAnswerable,
		}
	}

	return categories, nil
}

func (c *discussionClient) Create(_ ghrepo.Interface, _ CreateDiscussionInput) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) Update(_ ghrepo.Interface, _ UpdateDiscussionInput) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) Close(_ ghrepo.Interface, _ string, _ CloseReason) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) Reopen(_ ghrepo.Interface, _ string) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) AddComment(_ ghrepo.Interface, _ string, _ string, _ string) (*DiscussionComment, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) Lock(_ ghrepo.Interface, _ string, _ string) error {
	return fmt.Errorf("not implemented")
}

func (c *discussionClient) Unlock(_ ghrepo.Interface, _ string) error {
	return fmt.Errorf("not implemented")
}

func (c *discussionClient) MarkAnswer(_ ghrepo.Interface, _ string) error {
	return fmt.Errorf("not implemented")
}

func (c *discussionClient) UnmarkAnswer(_ ghrepo.Interface, _ string) error {
	return fmt.Errorf("not implemented")
}
