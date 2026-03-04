package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cli/cli/v2/internal/ghrepo"
)

// IssuesPayload represents categorized issue results for the current user.
type IssuesPayload struct {
	Assigned  IssuesAndTotalCount
	Mentioned IssuesAndTotalCount
	Authored  IssuesAndTotalCount
}

// IssuesAndTotalCount represents a list of issues along with pagination metadata.
type IssuesAndTotalCount struct {
	Issues       []Issue
	TotalCount   int
	SearchCapped bool
}

// Issue represents a GitHub issue or pull request.
type Issue struct {
	Typename         string `json:"__typename"`
	ID               string
	Number           int
	Title            string
	URL              string
	State            string
	StateReason      string
	Closed           bool
	Body             string
	ActiveLockReason string
	Locked           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ClosedAt         *time.Time
	Comments         Comments
	Author           Author
	Assignees        Assignees
	AssignedActors   AssignedActors
	Labels           Labels
	ProjectCards     ProjectCards
	ProjectItems     ProjectItems
	Milestone        *Milestone
	ReactionGroups   ReactionGroups
	IsPinned         bool

	ClosedByPullRequestsReferences ClosedByPullRequestsReferences
}

// ClosedByPullRequestsReferences represents pull requests that close an issue.
type ClosedByPullRequestsReferences struct {
	Nodes []struct {
		ID         string
		Number     int
		URL        string
		Repository struct {
			ID    string
			Name  string
			Owner struct {
				ID    string
				Login string
			}
		}
	}
	PageInfo struct {
		HasNextPage bool
		EndCursor   string
	}
}

// return values for Issue.Typename
const (
	TypeIssue       string = "Issue"
	TypePullRequest string = "PullRequest"
)

// IsPullRequest reports whether the issue is actually a pull request.
func (i Issue) IsPullRequest() bool {
	return i.Typename == TypePullRequest
}

// Assignees represents users assigned to an issue or pull request.
type Assignees struct {
	Nodes      []GitHubUser
	TotalCount int
}

// Logins returns the login usernames of all assignees.
func (a Assignees) Logins() []string {
	logins := make([]string, len(a.Nodes))
	for i, a := range a.Nodes {
		logins[i] = a.Login
	}
	return logins
}

// AssignedActors represents actors (users or bots) assigned to an issue or pull request.
type AssignedActors struct {
	Nodes      []Actor
	TotalCount int
}

// Logins returns the login usernames of all assigned actors.
func (a AssignedActors) Logins() []string {
	logins := make([]string, len(a.Nodes))
	for i, a := range a.Nodes {
		logins[i] = a.Login
	}
	return logins
}

// DisplayNames returns a list of display names for the assigned actors.
func (a AssignedActors) DisplayNames() []string {
	// These display names are used for populating the "default" assigned actors
	// from the AssignedActors type. But, this is only one piece of the puzzle
	// as later, other queries will fetch the full list of possible assignable
	// actors from the repository, and the two lists will be reconciled.
	//
	// It's important that the display names are the same between the defaults
	// (the values returned here) and the full list (the values returned by
	// other repository queries). Any discrepancy would result in an
	// "invalid default", which means an assigned actor will not be matched
	// to an assignable actor and not presented as a "default" selection.
	// Not being presented as a default would cause the actor to be potentially
	// unassigned if the edits were submitted.
	//
	// To prevent this, we need shared logic to look up an actor's display name.
	// However, our API types between assignedActors and the full list of
	// assignableActors are different. So, as an attempt to maintain
	// consistency we convert the assignedActors to the same types as the
	// repository's assignableActors, treating the assignableActors DisplayName
	// methods as the sources of truth.
	// TODO KW: make this comment less of a wall of text if needed.
	var displayNames []string
	for _, a := range a.Nodes {
		if a.TypeName == "User" {
			u := NewAssignableUser(
				a.ID,
				a.Login,
				a.Name,
			)
			displayNames = append(displayNames, u.DisplayName())
		} else if a.TypeName == "Bot" {
			b := NewAssignableBot(
				a.ID,
				a.Login,
			)
			displayNames = append(displayNames, b.DisplayName())
		}
	}
	return displayNames
}

// Labels represents the set of labels applied to an issue or pull request.
type Labels struct {
	Nodes      []IssueLabel
	TotalCount int
}

// Names returns the names of all labels.
func (l Labels) Names() []string {
	names := make([]string, len(l.Nodes))
	for i, l := range l.Nodes {
		names[i] = l.Name
	}
	return names
}

// ProjectCards represents classic project cards associated with an issue or pull request.
type ProjectCards struct {
	Nodes      []*ProjectInfo
	TotalCount int
}

// ProjectItems represents ProjectV2 items associated with an issue or pull request.
type ProjectItems struct {
	Nodes      []*ProjectV2Item
	TotalCount int
}

// ProjectInfo represents a classic project card's project and column names.
type ProjectInfo struct {
	Project struct {
		Name string `json:"name"`
	} `json:"project"`
	Column struct {
		Name string `json:"name"`
	} `json:"column"`
}

// ProjectV2Item represents an item within a GitHub ProjectV2.
type ProjectV2Item struct {
	ID      string `json:"id"`
	Project ProjectV2ItemProject
	Status  ProjectV2ItemStatus
}

// ProjectV2ItemProject represents the project associated with a ProjectV2 item.
type ProjectV2ItemProject struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ProjectV2ItemStatus represents the status field of a ProjectV2 item.
type ProjectV2ItemStatus struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
}

// ProjectNames returns the project names from all classic project cards.
func (p ProjectCards) ProjectNames() []string {
	names := make([]string, len(p.Nodes))
	for i, c := range p.Nodes {
		names[i] = c.Project.Name
	}
	return names
}

// ProjectTitles returns the project titles from all ProjectV2 items.
func (p ProjectItems) ProjectTitles() []string {
	titles := make([]string, len(p.Nodes))
	for i, c := range p.Nodes {
		titles[i] = c.Project.Title
	}
	return titles
}

// Milestone represents a GitHub milestone.
type Milestone struct {
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueOn       *time.Time `json:"dueOn"`
}

// IssuesDisabledError indicates that issues are disabled for a repository.
type IssuesDisabledError struct {
	error
}

// Owner represents the owner of a GitHub repository.
type Owner struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Login string `json:"login"`
}

// Author represents the author of an issue, pull request, or comment.
type Author struct {
	ID    string
	Name  string
	Login string
}

// MarshalJSON implements the json.Marshaler interface for Author.
func (author Author) MarshalJSON() ([]byte, error) {
	if author.ID == "" {
		return json.Marshal(map[string]interface{}{
			"is_bot": true,
			"login":  "app/" + author.Login,
		})
	}
	return json.Marshal(map[string]interface{}{
		"is_bot": false,
		"login":  author.Login,
		"id":     author.ID,
		"name":   author.Name,
	})
}

// CommentAuthor represents the author of a comment with only a login field.
type CommentAuthor struct {
	Login string `json:"login"`
	// Unfortunately, there is no easy way to add "id" and "name" fields to this struct because it's being
	// used in both shurcool-graphql type queries and string-based queries where the response gets parsed
	// by an ordinary JSON decoder that doesn't understand "graphql" directives via struct tags.
	//	User  *struct {
	//		ID   string
	//		Name string
	//	} `graphql:"... on User"`
}

// IssueCreate creates an issue in a GitHub repository
func IssueCreate(client *Client, repo *Repository, params map[string]interface{}) (*Issue, error) {
	query := `
	mutation IssueCreate($input: CreateIssueInput!) {
		createIssue(input: $input) {
			issue {
				id
				url
			}
		}
	}`

	inputParams := map[string]interface{}{
		"repositoryId": repo.ID,
	}
	for key, val := range params {
		switch key {
		case "assigneeIds", "body", "issueTemplate", "labelIds", "milestoneId", "projectIds", "repositoryId", "title":
			inputParams[key] = val
		case "projectV2Ids":
		default:
			return nil, fmt.Errorf("invalid IssueCreate mutation parameter %s", key)
		}
	}
	variables := map[string]interface{}{
		"input": inputParams,
	}

	result := struct {
		CreateIssue struct {
			Issue Issue
		}
	}{}

	err := client.GraphQL(repo.RepoHost(), query, variables, &result)
	if err != nil {
		return nil, err
	}
	issue := &result.CreateIssue.Issue

	// projectV2 parameters aren't supported in the `createIssue` mutation,
	// so add them after the issue has been created.
	projectV2Ids, ok := params["projectV2Ids"].([]string)
	if ok {
		projectItems := make(map[string]string, len(projectV2Ids))
		for _, p := range projectV2Ids {
			projectItems[p] = issue.ID
		}
		err = UpdateProjectV2Items(client, repo, projectItems, nil)
		if err != nil {
			return issue, err
		}
	}

	return issue, nil
}

// IssueStatusOptions specifies options for querying the current user's issue status.
type IssueStatusOptions struct {
	Username string
	Fields   []string
}

// IssueStatus returns issues assigned to, mentioning, or authored by the current user.
func IssueStatus(client *Client, repo ghrepo.Interface, options IssueStatusOptions) (*IssuesPayload, error) {
	type response struct {
		Repository struct {
			Assigned struct {
				TotalCount int
				Nodes      []Issue
			}
			Mentioned struct {
				TotalCount int
				Nodes      []Issue
			}
			Authored struct {
				TotalCount int
				Nodes      []Issue
			}
			HasIssuesEnabled bool
		}
	}

	fragments := fmt.Sprintf("fragment issue on Issue{%s}", IssueGraphQL(options.Fields))
	query := fragments + `
	query IssueStatus($owner: String!, $repo: String!, $viewer: String!, $per_page: Int = 10) {
		repository(owner: $owner, name: $repo) {
			hasIssuesEnabled
			assigned: issues(filterBy: {assignee: $viewer, states: OPEN}, first: $per_page, orderBy: {field: UPDATED_AT, direction: DESC}) {
				totalCount
				nodes {
					...issue
				}
			}
			mentioned: issues(filterBy: {mentioned: $viewer, states: OPEN}, first: $per_page, orderBy: {field: UPDATED_AT, direction: DESC}) {
				totalCount
				nodes {
					...issue
				}
			}
			authored: issues(filterBy: {createdBy: $viewer, states: OPEN}, first: $per_page, orderBy: {field: UPDATED_AT, direction: DESC}) {
				totalCount
				nodes {
					...issue
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":  repo.RepoOwner(),
		"repo":   repo.RepoName(),
		"viewer": options.Username,
	}

	var resp response
	err := client.GraphQL(repo.RepoHost(), query, variables, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Repository.HasIssuesEnabled {
		return nil, fmt.Errorf("the '%s' repository has disabled issues", ghrepo.FullName(repo))
	}

	payload := IssuesPayload{
		Assigned: IssuesAndTotalCount{
			Issues:     resp.Repository.Assigned.Nodes,
			TotalCount: resp.Repository.Assigned.TotalCount,
		},
		Mentioned: IssuesAndTotalCount{
			Issues:     resp.Repository.Mentioned.Nodes,
			TotalCount: resp.Repository.Mentioned.TotalCount,
		},
		Authored: IssuesAndTotalCount{
			Issues:     resp.Repository.Authored.Nodes,
			TotalCount: resp.Repository.Authored.TotalCount,
		},
	}

	return &payload, nil
}

// Link returns the URL of the issue.
func (i Issue) Link() string {
	return i.URL
}

// Identifier returns the unique ID of the issue.
func (i Issue) Identifier() string {
	return i.ID
}

// CurrentUserComments returns comments on the issue authored by the current user.
func (i Issue) CurrentUserComments() []Comment {
	return i.Comments.CurrentUserComments()
}
