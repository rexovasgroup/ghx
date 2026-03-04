package browser

import (
	"io"

	ghBrowser "github.com/cli/go-gh/v2/pkg/browser"
)

// Browser defines the interface for opening URLs in a browser.
type Browser interface {
	Browse(string) error
}

// New creates and returns a new instance with default settings.
func New(launcher string, stdout, stderr io.Writer) Browser {
	b := ghBrowser.New(launcher, stdout, stderr)
	return b
}
