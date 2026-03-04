package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/git"
	fd "github.com/cli/cli/v2/internal/featuredetection"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/githubtemplate"
	"github.com/shurcooL/githubv4"
)

type issueTemplate struct {
	Gname  string `graphql:"name"`
	Gbody  string `graphql:"body"`
	Gtitle string `graphql:"title"`
}

type pullRequestTemplate struct {
	Gname string `graphql:"filename"`
	Gbody string `graphql:"body"`
}

// Name returns the display name of the issue template.
func (t *issueTemplate) Name() string {
	return t.Gname
}

// NameForSubmit returns the template name to include when submitting an issue.
func (t *issueTemplate) NameForSubmit() string {
	return t.Gname
}

// Body returns the content of the issue template as bytes.
func (t *issueTemplate) Body() []byte {
	return []byte(t.Gbody)
}

// Title returns the default title of the issue template.
func (t *issueTemplate) Title() string {
	return t.Gtitle
}

// Name returns the filename of the pull request template.
func (t *pullRequestTemplate) Name() string {
	return t.Gname
}

// NameForSubmit returns an empty string since pull request templates do not have a submission name.
func (t *pullRequestTemplate) NameForSubmit() string {
	return ""
}

// Body returns the content of the pull request template as bytes.
func (t *pullRequestTemplate) Body() []byte {
	return []byte(t.Gbody)
}

// Title returns an empty string since pull request templates do not have a default title.
func (t *pullRequestTemplate) Title() string {
	return ""
}

func listIssueTemplates(httpClient *http.Client, repo ghrepo.Interface) ([]Template, error) {
	var query struct {
		Repository struct {
			IssueTemplates []issueTemplate
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(repo.RepoOwner()),
		"name":  githubv4.String(repo.RepoName()),
	}

	gql := api.NewClientFromHTTP(httpClient)

	err := gql.Query(repo.RepoHost(), "IssueTemplates", &query, variables)
	if err != nil {
		return nil, err
	}

	ts := query.Repository.IssueTemplates
	templates := make([]Template, len(ts))
	for i := range templates {
		templates[i] = &ts[i]
	}

	return templates, nil
}

func listPullRequestTemplates(httpClient *http.Client, repo ghrepo.Interface) ([]Template, error) {
	var query struct {
		Repository struct {
			PullRequestTemplates []pullRequestTemplate
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(repo.RepoOwner()),
		"name":  githubv4.String(repo.RepoName()),
	}

	gql := api.NewClientFromHTTP(httpClient)

	err := gql.Query(repo.RepoHost(), "PullRequestTemplates", &query, variables)
	if err != nil {
		return nil, err
	}

	ts := query.Repository.PullRequestTemplates
	templates := make([]Template, len(ts))
	for i := range templates {
		templates[i] = &ts[i]
	}

	return templates, nil
}

// Template defines the interface for issue and pull request templates.
type Template interface {
	Name() string
	NameForSubmit() string
	Body() []byte
	Title() string
}

type iprompter interface {
	Select(string, string, []string) (int, error)
}

type templateManager struct {
	repo       ghrepo.Interface
	rootDir    string
	allowFS    bool
	isPR       bool
	httpClient *http.Client
	detector   fd.Detector
	prompter   iprompter

	templates      []Template
	legacyTemplate Template

	didFetch   bool
	fetchError error
}

// NewTemplateManager creates a templateManager that discovers issue or PR templates from the API and optionally the filesystem.
func NewTemplateManager(httpClient *http.Client, repo ghrepo.Interface, p iprompter, dir string, allowFS bool, isPR bool) *templateManager {
	cachedClient := api.NewCachedHTTPClient(httpClient, time.Hour*24)
	return &templateManager{
		repo:       repo,
		rootDir:    dir,
		allowFS:    allowFS,
		isPR:       isPR,
		httpClient: httpClient,
		prompter:   p,
		detector:   fd.NewDetector(cachedClient, repo.RepoHost()),
	}
}

func (m *templateManager) hasAPI() (bool, error) {
	if !m.isPR {
		return true, nil
	}

	features, err := m.detector.RepositoryFeatures()
	if err != nil {
		return false, err
	}

	return features.PullRequestTemplateQuery, nil
}

// HasTemplates reports whether any templates are available for the repository.
func (m *templateManager) HasTemplates() (bool, error) {
	if err := m.memoizedFetch(); err != nil {
		return false, err
	}
	return len(m.templates) > 0, nil
}

// LegacyBody returns the body of the legacy template, or nil if no legacy template exists.
func (m *templateManager) LegacyBody() []byte {
	if m.legacyTemplate == nil {
		return nil
	}
	return m.legacyTemplate.Body()
}

// Choose prompts the user to interactively select a template from the available options.
func (m *templateManager) Choose() (Template, error) {
	if err := m.memoizedFetch(); err != nil {
		return nil, err
	}
	if len(m.templates) == 0 {
		return nil, nil
	}

	names := make([]string, len(m.templates))
	for i, t := range m.templates {
		names[i] = t.Name()
	}

	blankOption := "Open a blank issue"
	if m.isPR {
		blankOption = "Open a blank pull request"
	}

	selectedOption, err := m.prompter.Select("Choose a template", "", append(names, blankOption))
	if err != nil {
		return nil, fmt.Errorf("could not prompt: %w", err)
	}

	if selectedOption == len(names) {
		return nil, nil
	}
	return m.templates[selectedOption], nil
}

// Select returns the template matching the given name, or an error if not found.
func (m *templateManager) Select(name string) (Template, error) {
	if err := m.memoizedFetch(); err != nil {
		return nil, err
	}

	if len(m.templates) == 0 {
		return nil, errors.New("no templates found")
	}

	for _, t := range m.templates {
		if t.Name() == name {
			return t, nil
		}
	}

	return nil, fmt.Errorf("template %q not found", name)
}

func (m *templateManager) memoizedFetch() error {
	if m.didFetch {
		return m.fetchError
	}
	m.fetchError = m.fetch()
	m.didFetch = true
	return m.fetchError
}

func (m *templateManager) fetch() error {
	hasAPI, err := m.hasAPI()
	if err != nil {
		return err
	}

	if hasAPI {
		lister := listIssueTemplates
		if m.isPR {
			lister = listPullRequestTemplates
		}
		templates, err := lister(m.httpClient, m.repo)
		if err != nil {
			return err
		}
		m.templates = templates
	}

	if !m.allowFS {
		return nil
	}

	dir := m.rootDir
	if dir == "" {
		var err error
		gitClient := &git.Client{}
		dir, err = gitClient.ToplevelDir(context.Background())
		if err != nil {
			//nolint:nilerr // intentional, abort silently
			return nil
		}
	}

	filePattern := "ISSUE_TEMPLATE"
	if m.isPR {
		filePattern = "PULL_REQUEST_TEMPLATE"
	}

	if !hasAPI {
		issueTemplates := githubtemplate.FindNonLegacy(dir, filePattern)
		m.templates = make([]Template, len(issueTemplates))
		for i, t := range issueTemplates {
			m.templates[i] = &filesystemTemplate{path: t}
		}
	}

	if legacyTemplate := githubtemplate.FindLegacy(dir, filePattern); legacyTemplate != "" {
		m.legacyTemplate = &filesystemTemplate{path: legacyTemplate}
	}

	return nil
}

type filesystemTemplate struct {
	path string
}

// Name returns the display name extracted from the filesystem template file.
func (t *filesystemTemplate) Name() string {
	return githubtemplate.ExtractName(t.path)
}

// NameForSubmit returns an empty string since filesystem templates do not have a submission name.
func (t *filesystemTemplate) NameForSubmit() string {
	return ""
}

// Body returns the contents of the filesystem template file.
func (t *filesystemTemplate) Body() []byte {
	return githubtemplate.ExtractContents(t.path)
}

// Title returns the title extracted from the filesystem template file.
func (t *filesystemTemplate) Title() string {
	return githubtemplate.ExtractTitle(t.path)
}
