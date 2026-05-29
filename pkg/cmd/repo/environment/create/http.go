package create

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/repo/environment/shared"
)

type EnvironmentCreator struct {
	HTTPClient *http.Client
}

type EnvironmentCreateRequest struct {
	WaitTimer              *int                         `json:"wait_timer,omitempty"`
	PreventSelfReview      *bool                        `json:"prevent_self_review,omitempty"`
	Reviewers              []ReviewerRequest            `json:"reviewers,omitempty"`
	DeploymentBranchPolicy *shared.DeploymentBranchPolicy `json:"deployment_branch_policy,omitempty"`
}

type ReviewerRequest struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

func (e *EnvironmentCreator) CreateOrUpdate(repo ghrepo.Interface, name string, request EnvironmentCreateRequest) (*shared.Environment, error) {
	path := fmt.Sprintf("repos/%s/%s/environments/%s", repo.RepoOwner(), repo.RepoName(), name)
	url := ghinstance.RESTPrefix(repo.RepoHost()) + path

	requestByte, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	requestBody := bytes.NewReader(requestByte)

	req, err := http.NewRequest(http.MethodPut, url, requestBody)
	if err != nil {
		return nil, err
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return nil, api.HandleHTTPError(resp)
	}

	var env shared.Environment
	err = json.NewDecoder(resp.Body).Decode(&env)
	if err != nil {
		return nil, err
	}

	return &env, nil
}

func (e *EnvironmentCreator) CreateOrUpdateRaw(repo ghrepo.Interface, name string, body []byte) (*shared.Environment, error) {
	path := fmt.Sprintf("repos/%s/%s/environments/%s", repo.RepoOwner(), repo.RepoName(), name)
	url := ghinstance.RESTPrefix(repo.RepoHost()) + path

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return nil, api.HandleHTTPError(resp)
	}

	var env shared.Environment
	err = json.NewDecoder(resp.Body).Decode(&env)
	if err != nil {
		return nil, err
	}

	return &env, nil
}
