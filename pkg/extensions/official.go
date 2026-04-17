package extensions

import (
	"strings"

	"github.com/cli/cli/v2/internal/ghrepo"
)

// OfficialExtension describes a GitHub-owned CLI extension that can be
// suggested to users when they invoke an unknown command.
type OfficialExtension struct {
	Name  string
	Owner string
	Repo  string
}

// Repository returns a ghrepo.Interface pinned to github.com so that GHES
// users install from github.com rather than their enterprise host.
func (e *OfficialExtension) Repository() ghrepo.Interface {
	return ghrepo.NewWithHost(e.Owner, e.Repo, "github.com")
}

// OfficialExtensions is the registry of GitHub-owned extensions that gh will
// offer to install when the user invokes the corresponding command name.
var OfficialExtensions = []OfficialExtension{
	{Name: "aw", Owner: "github", Repo: "gh-aw"},
	{Name: "stack", Owner: "github", Repo: "gh-stack"},
}

// IsOfficial reports whether the given extension command name matches an
// entry in the OfficialExtensions registry. Only the name is checked
// because the name is the only value that can reach telemetry:
// cmdutil.RecordTelemetry records cmd.CommandPath() (plus parsed flag
// names, which for extensions are empty because DisableFlagParsing is set).
// A user running `gh <name>` therefore only emits telemetry for <name>
// when the registered cobra command carries that string, which is either
// a hard-coded stub name from this registry or the filename of an
// installed `gh-<name>` binary. Owner and host never reach telemetry, so
// they are not part of the check.
//
// Comparison is case-insensitive because extension names come from
// filenames and installed extensions preserve whatever casing was used at
// install time.
func IsOfficial(name string) bool {
	for _, ext := range OfficialExtensions {
		if strings.EqualFold(ext.Name, name) {
			return true
		}
	}
	return false
}
