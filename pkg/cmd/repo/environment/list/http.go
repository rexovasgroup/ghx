package list

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/repo/environment/shared"
)

type EnvironmentLister struct {
	HTTPClient *http.Client
}

func (e *EnvironmentLister) List(repo ghrepo.Interface) ([]shared.Environment, int, error) {
	path := fmt.Sprintf("repos/%s/%s/environments", repo.RepoOwner(), repo.RepoName())
	url := ghinstance.RESTPrefix(repo.RepoHost()) + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return nil, 0, api.HandleHTTPError(resp)
	}

	var response shared.EnvironmentListResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, 0, err
	}

	return response.Environments, response.TotalCount, nil
}
