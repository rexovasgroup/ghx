package delete

import (
	"fmt"
	"net/http"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/internal/ghrepo"
)

type EnvironmentDeleter struct {
	HTTPClient *http.Client
}

func (e *EnvironmentDeleter) Delete(repo ghrepo.Interface, name string) error {
	path := fmt.Sprintf("repos/%s/%s/environments/%s", repo.RepoOwner(), repo.RepoName(), name)
	url := ghinstance.RESTPrefix(repo.RepoHost()) + path
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return api.HandleHTTPError(resp)
	}

	return nil
}
