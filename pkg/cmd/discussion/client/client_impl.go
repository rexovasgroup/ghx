package client

import (
	"fmt"
	"net/http"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
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

func (c *discussionClient) List(_ ghrepo.Interface, _ ListFilters, _ int) ([]Discussion, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (c *discussionClient) Search(_ ghrepo.Interface, _ SearchFilters, _ int) ([]Discussion, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (c *discussionClient) GetByNumber(_ ghrepo.Interface, _ int) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) GetWithComments(_ ghrepo.Interface, _ int, _ int, _ string) (*Discussion, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *discussionClient) ListCategories(_ ghrepo.Interface) ([]DiscussionCategory, error) {
	return nil, fmt.Errorf("not implemented")
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
