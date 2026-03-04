package featuredetection

import "github.com/cli/cli/v2/internal/gh"

// DisabledDetectorMock is a mock Detector that returns zero-value features for all queries.
type DisabledDetectorMock struct{}

// IssueFeatures returns empty issue features.
func (md *DisabledDetectorMock) IssueFeatures() (IssueFeatures, error) {
	return IssueFeatures{}, nil
}

// PullRequestFeatures returns empty pull request features.
func (md *DisabledDetectorMock) PullRequestFeatures() (PullRequestFeatures, error) {
	return PullRequestFeatures{}, nil
}

// RepositoryFeatures returns empty repository features.
func (md *DisabledDetectorMock) RepositoryFeatures() (RepositoryFeatures, error) {
	return RepositoryFeatures{}, nil
}

// ProjectsV1 returns unsupported for ProjectsV1.
func (md *DisabledDetectorMock) ProjectsV1() gh.ProjectsV1Support {
	return gh.ProjectsV1Unsupported
}

// ProjectFeatures returns empty project features.
func (md *DisabledDetectorMock) ProjectFeatures() (ProjectFeatures, error) {
	return ProjectFeatures{}, nil
}

// SearchFeatures returns search features with advanced issue search disabled.
func (md *DisabledDetectorMock) SearchFeatures() (SearchFeatures, error) {
	return advancedIssueSearchNotSupported, nil
}

// ReleaseFeatures returns empty release features.
func (md *DisabledDetectorMock) ReleaseFeatures() (ReleaseFeatures, error) {
	return ReleaseFeatures{}, nil
}

// ActionsFeatures returns empty actions features.
func (md *DisabledDetectorMock) ActionsFeatures() (ActionsFeatures, error) {
	return ActionsFeatures{}, nil
}

// EnabledDetectorMock is a mock Detector that returns all features as enabled.
type EnabledDetectorMock struct{}

// IssueFeatures returns all issue features enabled.
func (md *EnabledDetectorMock) IssueFeatures() (IssueFeatures, error) {
	return allIssueFeatures, nil
}

// PullRequestFeatures returns all pull request features enabled.
func (md *EnabledDetectorMock) PullRequestFeatures() (PullRequestFeatures, error) {
	return allPullRequestFeatures, nil
}

// RepositoryFeatures returns all repository features enabled.
func (md *EnabledDetectorMock) RepositoryFeatures() (RepositoryFeatures, error) {
	return allRepositoryFeatures, nil
}

// ProjectsV1 returns supported for ProjectsV1.
func (md *EnabledDetectorMock) ProjectsV1() gh.ProjectsV1Support {
	return gh.ProjectsV1Supported
}

// ProjectFeatures returns all project features enabled.
func (md *EnabledDetectorMock) ProjectFeatures() (ProjectFeatures, error) {
	return allProjectFeatures, nil
}

// SearchFeatures returns search features with advanced issue search disabled.
func (md *EnabledDetectorMock) SearchFeatures() (SearchFeatures, error) {
	return advancedIssueSearchNotSupported, nil
}

// ReleaseFeatures returns release features with immutable releases enabled.
func (md *EnabledDetectorMock) ReleaseFeatures() (ReleaseFeatures, error) {
	return ReleaseFeatures{
		ImmutableReleases: true,
	}, nil
}

// ActionsFeatures returns actions features with dispatch run details enabled.
func (md *EnabledDetectorMock) ActionsFeatures() (ActionsFeatures, error) {
	return ActionsFeatures{
		DispatchRunDetails: true,
	}, nil
}

// AdvancedIssueSearchDetectorMock is a mock Detector with configurable search features.
type AdvancedIssueSearchDetectorMock struct {
	EnabledDetectorMock
	searchFeatures SearchFeatures
}

// SearchFeatures returns the configured search features.
func (md *AdvancedIssueSearchDetectorMock) SearchFeatures() (SearchFeatures, error) {
	return md.searchFeatures, nil
}

// AdvancedIssueSearchUnsupported returns a mock detector where advanced issue search is not supported.
func AdvancedIssueSearchUnsupported() *AdvancedIssueSearchDetectorMock {
	return &AdvancedIssueSearchDetectorMock{
		searchFeatures: advancedIssueSearchNotSupported,
	}
}

// AdvancedIssueSearchSupportedAsOptIn returns a mock detector where advanced issue search is opt-in.
func AdvancedIssueSearchSupportedAsOptIn() *AdvancedIssueSearchDetectorMock {
	return &AdvancedIssueSearchDetectorMock{
		searchFeatures: advancedIssueSearchSupportedAsOptIn,
	}
}

// AdvancedIssueSearchSupportedAsOnlyBackend returns a mock detector where advanced issue search is the only backend.
func AdvancedIssueSearchSupportedAsOnlyBackend() *AdvancedIssueSearchDetectorMock {
	return &AdvancedIssueSearchDetectorMock{
		searchFeatures: advancedIssueSearchSupportedAsOnlyBackend,
	}
}
