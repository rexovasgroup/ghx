// Package gheslatest contains the pr edit implementation for the latest supported GHES version.
// This version uses the older APIs that don't support features like ActorIsAssignable.
//
// When this GHES version reaches EOL, this package can be deleted and removed from the
// VersionedCommand configuration in edit.go.
package gheslatest

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cli/cli/v2/api"
	fd "github.com/cli/cli/v2/internal/featuredetection"
	"github.com/cli/cli/v2/internal/ghrepo"
	editshared "github.com/cli/cli/v2/pkg/cmd/pr/edit/editshared"
	"github.com/cli/cli/v2/pkg/cmd/pr/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/set"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewRunE returns the RunE function for gheslatest.
// This includes full flag parsing and validation.
func NewRunE(f *cmdutil.Factory, opts *editshared.EditOptions) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := opts.ParseFlags(cmd, args, f); err != nil {
			return err
		}
		return run(opts)
	}
}

// run is the gheslatest implementation of pr edit.
// It uses the older assignees field instead of assignedActors.
func run(opts *editshared.EditOptions) error {
	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}

	if opts.Detector == nil {
		baseRepo, err := opts.BaseRepo()
		if err != nil {
			return err
		}

		cachedClient := api.NewCachedHTTPClient(httpClient, time.Hour*24)
		opts.Detector = fd.NewDetector(cachedClient, baseRepo.RepoHost())
	}

	// gheslatest uses "assignees" field, not "assignedActors"
	findOptions := shared.FindOptions{
		Selector: opts.SelectorArg,
		Fields:   []string{"id", "author", "url", "title", "body", "baseRefName", "reviewRequests", "assignees", "labels", "projectCards", "projectItems", "milestone"},
		Detector: opts.Detector,
	}

	pr, repo, err := opts.Finder.Find(findOptions)
	if err != nil {
		return err
	}

	editable := opts.Editable
	editable.Reviewers.Allowed = true
	editable.Title.Default = pr.Title
	editable.Body.Default = pr.Body
	editable.Base.Default = pr.BaseRefName
	editable.Reviewers.Default = pr.ReviewRequests.Logins()
	// gheslatest: use old assignees field
	editable.Assignees.Default = pr.Assignees.Logins()
	editable.Labels.Default = pr.Labels.Names()
	editable.Projects.Default = append(pr.ProjectCards.ProjectNames(), pr.ProjectItems.ProjectTitles()...)
	projectItems := map[string]string{}
	for _, n := range pr.ProjectItems.Nodes {
		projectItems[n.Project.ID] = n.ID
	}
	editable.Projects.ProjectItems = projectItems
	if pr.Milestone != nil {
		editable.Milestone.Default = pr.Milestone.Title
	}

	if opts.Interactive {
		err = opts.Surveyor.FieldsToEdit(&editable)
		if err != nil {
			return err
		}
	}

	apiClient := api.NewClientFromHTTP(httpClient)

	opts.IO.StartProgressIndicator()
	err = opts.Fetcher.EditableOptionsFetch(apiClient, repo, &editable, opts.Detector.ProjectsV1())
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.Interactive {
		// Remove PR author from reviewer options
		if editable.Reviewers.Edited {
			s := set.NewStringSet()
			s.AddValues(editable.Reviewers.Options)
			s.Remove(pr.Author.Login)
			editable.Reviewers.Options = s.ToSlice()
		}

		editorCommand, err := opts.EditorRetriever.Retrieve()
		if err != nil {
			return err
		}
		err = opts.Surveyor.EditFields(&editable, editorCommand)
		if err != nil {
			return err
		}
	}

	opts.IO.StartProgressIndicator()
	err = updatePullRequest(httpClient, repo, pr.ID, pr.Number, editable)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	fmt.Fprintln(opts.IO.Out, pr.URL)

	return nil
}

func updatePullRequest(httpClient *http.Client, repo ghrepo.Interface, id string, number int, editable shared.Editable) error {
	var wg errgroup.Group
	wg.Go(func() error {
		return shared.UpdateIssue(httpClient, repo, id, true, editable)
	})
	if editable.Reviewers.Edited {
		wg.Go(func() error {
			return updatePullRequestReviews(httpClient, repo, number, editable)
		})
	}
	return wg.Wait()
}

func updatePullRequestReviews(httpClient *http.Client, repo ghrepo.Interface, number int, editable shared.Editable) error {
	if !editable.Reviewers.Edited {
		return nil
	}

	if len(editable.Reviewers.Add) != 0 || len(editable.Reviewers.Remove) != 0 {
		s := set.NewStringSet()
		s.AddValues(editable.Reviewers.Add)
		s.AddValues(editable.Reviewers.Default)
		s.RemoveValues(editable.Reviewers.Remove)
		editable.Reviewers.Value = s.ToSlice()
	}

	addUsers, addTeams := editshared.PartitionUsersAndTeams(editable.Reviewers.Value)

	var toRemove []string
	for _, r := range editable.Reviewers.Default {
		if !contains(editable.Reviewers.Value, r) {
			toRemove = append(toRemove, r)
		}
	}
	removeUsers, removeTeams := editshared.PartitionUsersAndTeams(toRemove)

	client := api.NewClientFromHTTP(httpClient)
	wg := errgroup.Group{}
	wg.Go(func() error {
		return api.AddPullRequestReviews(client, repo, number, addUsers, addTeams)
	})
	wg.Go(func() error {
		return api.RemovePullRequestReviews(client, repo, number, removeUsers, removeTeams)
	})
	return wg.Wait()
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
