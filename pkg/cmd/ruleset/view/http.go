package view

import (
	"net/http"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/ruleset/shared"
)

func viewRepoRuleset(httpClient *http.Client, repo ghrepo.Interface, databaseId string) (*shared.RulesetREST, error) {
	return shared.GetRepoRuleset(httpClient, repo, databaseId)
}

func viewOrgRuleset(httpClient *http.Client, orgLogin string, databaseId string, host string) (*shared.RulesetREST, error) {
	return shared.GetOrgRuleset(httpClient, orgLogin, databaseId, host)
}
