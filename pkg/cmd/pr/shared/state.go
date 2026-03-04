package shared

import (
	"encoding/json"
	"fmt"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/pkg/iostreams"
)

type metadataStateType int

const (
	// IssueMetadata indicates that the metadata state is for an issue.
	IssueMetadata metadataStateType = iota
	// PRMetadata indicates that the metadata state is for a pull request.
	PRMetadata
)

// IssueMetadataState holds the editable metadata for an issue or pull request during creation or editing.
type IssueMetadataState struct {
	Type metadataStateType

	Draft          bool
	ActorAssignees bool

	Body  string
	Title string

	Template string

	Metadata      []string
	Reviewers     []string
	Assignees     []string
	Labels        []string
	ProjectTitles []string
	Milestones    []string

	MetadataResult *api.RepoMetadataResult

	dirty bool // whether user i/o has modified this
}

// MarkDirty flags the state as having been modified by user input.
func (tb *IssueMetadataState) MarkDirty() {
	tb.dirty = true
}

// IsDirty reports whether the state has been modified by user input or has metadata set.
func (tb *IssueMetadataState) IsDirty() bool {
	return tb.dirty || tb.HasMetadata()
}

// HasMetadata reports whether any reviewers, assignees, labels, projects, or milestones are set.
func (tb *IssueMetadataState) HasMetadata() bool {
	return len(tb.Reviewers) > 0 ||
		len(tb.Assignees) > 0 ||
		len(tb.Labels) > 0 ||
		len(tb.ProjectTitles) > 0 ||
		len(tb.Milestones) > 0
}

// FillFromJSON populates the given IssueMetadataState by reading and unmarshaling a JSON recovery file.
func FillFromJSON(io *iostreams.IOStreams, recoverFile string, state *IssueMetadataState) error {
	var data []byte
	var err error
	data, err = io.ReadUserFile(recoverFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", recoverFile, err)
	}

	err = json.Unmarshal(data, state)
	if err != nil {
		return fmt.Errorf("JSON parsing failure: %w", err)
	}

	return nil
}
