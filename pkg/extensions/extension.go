package extensions

import (
	"io"

	"github.com/cli/cli/v2/internal/ghrepo"
)

// ExtTemplateType represents the type of extension template.
type ExtTemplateType int

const (
	// GitTemplateType represents a git-based extension template.
	GitTemplateType ExtTemplateType = 0
	// GoBinTemplateType represents a Go binary extension template.
	GoBinTemplateType ExtTemplateType = 1
	// OtherBinTemplateType represents a precompiled binary extension template.
	OtherBinTemplateType ExtTemplateType = 2
)

//go:generate moq -rm -out extension_mock.go . Extension

// Extension defines the interface for a GitHub CLI extension.
type Extension interface {
	Name() string // Extension Name without gh-
	Path() string // Path to executable
	URL() string
	CurrentVersion() string
	LatestVersion() string
	IsPinned() bool
	UpdateAvailable() bool
	IsBinary() bool
	IsLocal() bool
	Owner() string
}

//go:generate moq -rm -out manager_mock.go . ExtensionManager

// ExtensionManager defines the interface for managing CLI extensions.
type ExtensionManager interface {
	List() []Extension
	Install(ghrepo.Interface, string) error
	InstallLocal(dir string) error
	Upgrade(name string, force bool) error
	Remove(name string) error
	Dispatch(args []string, stdin io.Reader, stdout, stderr io.Writer) (bool, error)
	Create(name string, tmplType ExtTemplateType) error
	EnableDryRunMode()
	UpdateDir(name string) string
}
