package git

import (
	"errors"
	"fmt"
)

// ErrNotOnAnyBranch indicates that the user is in detached HEAD state.
var ErrNotOnAnyBranch = errors.New("git: not on any branch")

// NotInstalled indicates that the git binary could not be found.
type NotInstalled struct {
	message string
	err     error
}

// Error returns the error message for NotInstalled.
func (e *NotInstalled) Error() string {
	return e.message
}

// Unwrap returns the underlying error of NotInstalled.
func (e *NotInstalled) Unwrap() error {
	return e.err
}

// GitError wraps an error from a failed git command with exit code and stderr output.
type GitError struct {
	ExitCode int
	Stderr   string
	err      error
}

// Error returns a formatted message including stderr or the underlying error.
func (ge *GitError) Error() string {
	if ge.Stderr == "" {
		return fmt.Sprintf("failed to run git: %v", ge.err)
	}
	return fmt.Sprintf("failed to run git: %s", ge.Stderr)
}

// Unwrap returns the underlying error of GitError.
func (ge *GitError) Unwrap() error {
	return ge.err
}
