package extension

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cli/cli/v2/git"
	"github.com/cli/cli/v2/internal/ghrepo"
	"gopkg.in/yaml.v3"
)

const manifestName = "manifest.yml"

// ExtensionKind indicates the type of a gh CLI extension.
type ExtensionKind int

const (
	// GitKind represents a git-based extension.
	GitKind ExtensionKind = iota
	// BinaryKind represents a precompiled binary extension.
	BinaryKind
	// LocalKind represents a locally developed extension.
	LocalKind
)

// Extension represents an installed gh CLI extension.
type Extension struct {
	path       string
	kind       ExtensionKind
	gitClient  gitClient
	httpClient *http.Client

	mu sync.RWMutex

	// These fields get resolved dynamically:
	url            string
	isPinned       *bool
	currentVersion string
	latestVersion  string
	owner          string
}

// Name returns the extension name without the "gh-" prefix.
func (e *Extension) Name() string {
	return strings.TrimPrefix(filepath.Base(e.path), "gh-")
}

// Path returns the filesystem path to the extension executable.
func (e *Extension) Path() string {
	return e.path
}

// IsLocal returns true if the extension is a locally developed extension.
func (e *Extension) IsLocal() bool {
	return e.kind == LocalKind
}

// IsBinary returns true if the extension is a precompiled binary.
func (e *Extension) IsBinary() bool {
	return e.kind == BinaryKind
}

// URL returns the repository URL of the extension.
func (e *Extension) URL() string {
	e.mu.RLock()
	if e.url != "" {
		defer e.mu.RUnlock()
		return e.url
	}
	e.mu.RUnlock()

	var url string
	switch e.kind {
	case LocalKind:
	case BinaryKind:
		if manifest, err := e.loadManifest(); err == nil {
			repo := ghrepo.NewWithHost(manifest.Owner, manifest.Name, manifest.Host)
			url = ghrepo.GenerateRepoURL(repo, "")
		}
	case GitKind:
		if remoteURL, err := e.gitClient.Config("remote.origin.url"); err == nil {
			url = strings.TrimSpace(string(remoteURL))
		}
	}

	e.mu.Lock()
	e.url = url
	e.mu.Unlock()

	return e.url
}

// CurrentVersion returns the currently installed version of the extension.
func (e *Extension) CurrentVersion() string {
	e.mu.RLock()
	if e.currentVersion != "" {
		defer e.mu.RUnlock()
		return e.currentVersion
	}
	e.mu.RUnlock()

	var currentVersion string
	switch e.kind {
	case LocalKind:
	case BinaryKind:
		if manifest, err := e.loadManifest(); err == nil {
			currentVersion = manifest.Tag
		}
	case GitKind:
		if sha, err := e.gitClient.CommandOutput([]string{"rev-parse", "HEAD"}); err == nil {
			currentVersion = string(bytes.TrimSpace(sha))
		}
	}

	e.mu.Lock()
	e.currentVersion = currentVersion
	e.mu.Unlock()

	return e.currentVersion
}

// LatestVersion returns the latest available version of the extension.
func (e *Extension) LatestVersion() string {
	e.mu.RLock()
	if e.latestVersion != "" {
		defer e.mu.RUnlock()
		return e.latestVersion
	}
	e.mu.RUnlock()

	var latestVersion string
	switch e.kind {
	case LocalKind:
	case BinaryKind:
		repo, err := ghrepo.FromFullName(e.URL())
		if err != nil {
			return ""
		}
		release, err := fetchLatestRelease(e.httpClient, repo)
		if err != nil {
			return ""
		}
		latestVersion = release.Tag
	case GitKind:
		if lsRemote, err := e.gitClient.CommandOutput([]string{"ls-remote", "origin", "HEAD"}); err == nil {
			latestVersion = string(bytes.SplitN(lsRemote, []byte("\t"), 2)[0])
		}
	}

	e.mu.Lock()
	e.latestVersion = latestVersion
	e.mu.Unlock()

	return e.latestVersion
}

// IsPinned returns true if the extension is pinned to a specific version.
func (e *Extension) IsPinned() bool {
	e.mu.RLock()
	if e.isPinned != nil {
		defer e.mu.RUnlock()
		return *e.isPinned
	}
	e.mu.RUnlock()

	var isPinned bool
	switch e.kind {
	case LocalKind:
	case BinaryKind:
		if manifest, err := e.loadManifest(); err == nil {
			isPinned = manifest.IsPinned
		}
	case GitKind:
		extDir := filepath.Dir(e.path)
		pinPath := filepath.Join(extDir, fmt.Sprintf(".pin-%s", e.CurrentVersion()))
		if _, err := os.Stat(pinPath); err == nil {
			isPinned = true
		} else {
			isPinned = false
		}
	}

	e.mu.Lock()
	e.isPinned = &isPinned
	e.mu.Unlock()

	return *e.isPinned
}

// Owner returns the GitHub owner of the extension repository.
func (e *Extension) Owner() string {
	e.mu.RLock()
	if e.owner != "" {
		defer e.mu.RUnlock()
		return e.owner
	}
	e.mu.RUnlock()

	var owner string
	switch e.kind {
	case LocalKind:
	case BinaryKind:
		if manifest, err := e.loadManifest(); err == nil {
			owner = manifest.Owner
		}
	case GitKind:
		if remoteURL, err := e.gitClient.Config("remote.origin.url"); err == nil {
			if url, err := git.ParseURL(strings.TrimSpace(string(remoteURL))); err == nil {
				if repo, err := ghrepo.FromURL(url); err == nil {
					owner = repo.RepoOwner()
				}
			}
		}
	}

	e.mu.Lock()
	e.owner = owner
	e.mu.Unlock()

	return e.owner
}

// UpdateAvailable returns true if a newer version of the extension exists.
func (e *Extension) UpdateAvailable() bool {
	if e.IsLocal() ||
		e.CurrentVersion() == "" ||
		e.LatestVersion() == "" ||
		e.CurrentVersion() == e.LatestVersion() {
		return false
	}
	return true
}

func (e *Extension) loadManifest() (binManifest, error) {
	var bm binManifest
	dir, _ := filepath.Split(e.Path())
	manifestPath := filepath.Join(dir, manifestName)
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return bm, fmt.Errorf("could not open %s for reading: %w", manifestPath, err)
	}
	err = yaml.Unmarshal(manifest, &bm)
	if err != nil {
		return bm, fmt.Errorf("could not parse %s: %w", manifestPath, err)
	}
	return bm, nil
}
