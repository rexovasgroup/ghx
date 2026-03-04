package api

import (
	"time"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/shurcooL/githubv4"
)

// PullRequestReviewState represents the type of review event for a pull request.
type PullRequestReviewState int

const (
	// ReviewApprove is a review that approves the pull request.
	ReviewApprove PullRequestReviewState = iota
	// ReviewRequestChanges is a review that requests changes on the pull request.
	ReviewRequestChanges
	// ReviewComment is a review that leaves a comment without approval or change request.
	ReviewComment
)

// PullRequestReviewInput defines the parameters for submitting a pull request review.
type PullRequestReviewInput struct {
	Body  string
	State PullRequestReviewState
}

// PullRequestReviews represents a paginated list of pull request reviews.
type PullRequestReviews struct {
	Nodes    []PullRequestReview
	PageInfo struct {
		HasNextPage bool
		EndCursor   string
	}
	TotalCount int
}

// PullRequestReview represents a single review on a pull request.
type PullRequestReview struct {
	ID                  string         `json:"id"`
	Author              CommentAuthor  `json:"author"`
	AuthorAssociation   string         `json:"authorAssociation"`
	Body                string         `json:"body"`
	SubmittedAt         *time.Time     `json:"submittedAt"`
	IncludesCreatedEdit bool           `json:"includesCreatedEdit"`
	ReactionGroups      ReactionGroups `json:"reactionGroups"`
	State               string         `json:"state"`
	URL                 string         `json:"url,omitempty"`
	Commit              Commit         `json:"commit"`
}

// AddReview submits a review for the given pull request via the GraphQL API.
func AddReview(client *Client, repo ghrepo.Interface, pr *PullRequest, input *PullRequestReviewInput) error {
	var mutation struct {
		AddPullRequestReview struct {
			ClientMutationID string
		} `graphql:"addPullRequestReview(input:$input)"`
	}

	state := githubv4.PullRequestReviewEventComment
	switch input.State {
	case ReviewApprove:
		state = githubv4.PullRequestReviewEventApprove
	case ReviewRequestChanges:
		state = githubv4.PullRequestReviewEventRequestChanges
	}

	body := githubv4.String(input.Body)
	variables := map[string]interface{}{
		"input": githubv4.AddPullRequestReviewInput{
			PullRequestID: pr.ID,
			Event:         &state,
			Body:          &body,
		},
	}

	return client.Mutate(repo.RepoHost(), "PullRequestReviewAdd", &mutation, variables)
}

// Identifier returns the unique ID of the review.
func (prr PullRequestReview) Identifier() string {
	return prr.ID
}

// AuthorLogin returns the login name of the review author.
func (prr PullRequestReview) AuthorLogin() string {
	return prr.Author.Login
}

// Association returns the author's association with the repository.
func (prr PullRequestReview) Association() string {
	return prr.AuthorAssociation
}

// Content returns the body text of the review.
func (prr PullRequestReview) Content() string {
	return prr.Body
}

// Created returns the time when the review was submitted.
func (prr PullRequestReview) Created() time.Time {
	if prr.SubmittedAt == nil {
		return time.Time{}
	}
	return *prr.SubmittedAt
}

// HiddenReason returns the reason the review was hidden; always empty for reviews.
func (prr PullRequestReview) HiddenReason() string {
	return ""
}

// IsEdited returns whether the review has been edited.
func (prr PullRequestReview) IsEdited() bool {
	return prr.IncludesCreatedEdit
}

// IsHidden returns whether the review is hidden; always false for reviews.
func (prr PullRequestReview) IsHidden() bool {
	return false
}

// Link returns the URL of the review.
func (prr PullRequestReview) Link() string {
	return prr.URL
}

// Reactions returns the reaction groups associated with the review.
func (prr PullRequestReview) Reactions() ReactionGroups {
	return prr.ReactionGroups
}

// Status returns the review state (e.g., APPROVED, CHANGES_REQUESTED).
func (prr PullRequestReview) Status() string {
	return prr.State
}
