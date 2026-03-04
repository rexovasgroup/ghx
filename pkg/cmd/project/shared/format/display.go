package format

import (
	"github.com/cli/cli/v2/pkg/cmd/project/shared/queries"
)

// ProjectState returns the display state string ("open" or "closed") for a project.
func ProjectState(project queries.Project) string {
	if project.Closed {
		return "closed"
	}
	return "open"
}

// ColorForProjectState returns the display color for a project based on its state.
func ColorForProjectState(project queries.Project) string {
	if project.Closed {
		return "gray"
	}
	return "green"
}
