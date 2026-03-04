package httpmock

import (
	"fmt"
)

// TODO: clean up methods in this file when there are no more callers

// StubRepoInfoResponse registers a stub for the RepositoryInfo GraphQL query.
func (r *Registry) StubRepoInfoResponse(owner, repo, branch string) {
	r.Register(
		GraphQL(`query RepositoryInfo\b`),
		StringResponse(fmt.Sprintf(`
		{ "data": { "repository": {
			"id": "REPOID",
			"name": "%s",
			"owner": {"login": "%s"},
			"description": "",
			"defaultBranchRef": {"name": "%s"},
			"hasIssuesEnabled": true,
			"viewerPermission": "WRITE"
		} } }
		`, repo, owner, branch)))
}

// StubRepoResponse registers a stub for the RepositoryNetwork GraphQL query with WRITE permission.
func (r *Registry) StubRepoResponse(owner, repo string) {
	r.StubRepoResponseWithPermission(owner, repo, "WRITE")
}

// StubRepoResponseWithPermission registers a stub for the RepositoryNetwork GraphQL query with the given permission.
func (r *Registry) StubRepoResponseWithPermission(owner, repo, permission string) {
	r.Register(GraphQL(`query RepositoryNetwork\b`), StringResponse(RepoNetworkStubResponse(owner, repo, "master", permission)))
}

// RepoNetworkStubResponse returns a JSON string representing a RepositoryNetwork GraphQL response.
func RepoNetworkStubResponse(owner, repo, defaultBranch, permission string) string {
	return fmt.Sprintf(`
		{ "data": { "repo_000": {
			"id": "REPOID",
			"name": "%s",
			"owner": {"login": "%s"},
			"defaultBranchRef": {
				"name": "%s"
			},
			"viewerPermission": "%s"
		} } }
	`, repo, owner, defaultBranch, permission)
}
