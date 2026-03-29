package shared

import (
	"fmt"
	"strings"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
)

// ResolveIssueRef parses an issue reference (number or URL) and returns its node ID.
func ResolveIssueRef(client *api.Client, baseRepo ghrepo.Interface, ref string) (string, error) {
	number, repo, err := ParseIssueFromArg(ref)
	if err != nil {
		return "", err
	}

	targetRepo := baseRepo
	if r, ok := repo.Value(); ok {
		targetRepo = r
	}

	return api.IssueNodeID(client, targetRepo, number)
}

// ResolveIssueTypeName resolves an issue type name to its ID by fetching
// available types from the repository. Returns an error listing available
// types if the name is not found.
func ResolveIssueTypeName(client *api.Client, repo ghrepo.Interface, typeName string) (string, error) {
	issueTypes, err := api.RepoIssueTypes(client, repo)
	if err != nil {
		return "", err
	}

	typeNames := make([]string, len(issueTypes))
	for i, t := range issueTypes {
		typeNames[i] = t.Name
		if strings.EqualFold(t.Name, typeName) {
			return t.ID, nil
		}
	}

	return "", fmt.Errorf("type %q not found; available types: %s", typeName, strings.Join(typeNames, ", "))
}
