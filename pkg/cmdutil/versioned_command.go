package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"

	ghauth "github.com/cli/go-gh/v2/pkg/auth"
)

// VersionedCommand provides a mechanism to register multiple implementations of a command
// that are selected at runtime based on the target host and its version.
//
// This allows complete encapsulation of version-specific logic rather than scattering
// conditionals throughout the codebase. When a GHES version reaches EOL, cleanup is
// simple: delete the variant package and remove it from the Variants map.
//
// The key insight is that we keep ONE command (with flags, help text, etc.) but swap
// the execution logic based on the detected host version. All variants share the same
// Options struct that was populated by flag parsing.
//
// Example usage:
//
//	func NewCmdEdit(f *cmdutil.Factory, runF func(*EditOptions) error) *cobra.Command {
//	    opts := &EditOptions{...}
//
//	    cmd := &cobra.Command{
//	        Use:   "edit",
//	        Short: "Edit a pull request",
//	        // ... flags bound to opts ...
//	    }
//
//	    vc := &cmdutil.VersionedCommand{
//	        Name: "edit",
//	        Cmd:  cmd,
//	        // Default runner for github.com and latest GHES
//	        DefaultRunE: func(cmd *cobra.Command, args []string) error {
//	            return cloudlatest.Run(opts)
//	        },
//	        Variants: map[string]VersionedVariant{
//	            "ghes-latest": {
//	                RunE:       func(cmd *cobra.Command, args []string) error { return gheslatest.Run(opts) },
//	                Constraint: cmdutil.GHESVersionConstraint("<", "3.17.0"),
//	            },
//	        },
//	        HostResolver: cmdutil.RepoHostResolver(opts.BaseRepo),
//	        HttpClient:   f.HttpClient,
//	    }
//
//	    return vc.Command()
//	}
type VersionedCommand struct {
	// Name is used for error messages
	Name string

	// Cmd is the cobra command with flags, help text, etc.
	// Its RunE will be replaced with version-routing logic.
	Cmd *cobra.Command

	// DefaultRunE is the execution logic for github.com and latest GHES versions
	DefaultRunE func(cmd *cobra.Command, args []string) error

	// Variants maps version identifiers (e.g., "ghes-latest") to their execution logic
	Variants map[string]VersionedVariant

	// HostResolver returns the target host. Called during RunE after flags are parsed.
	// Use RepoHostResolver() helper for the common case.
	HostResolver func() (string, error)

	// HttpClient is used to query the GHES version via the /meta endpoint
	HttpClient func() (*http.Client, error)
}

// VersionedVariant represents a version-specific command implementation
type VersionedVariant struct {
	// RunE is the execution logic for this variant
	RunE func(cmd *cobra.Command, args []string) error

	// Constraint is a function that returns true if this variant should be used
	// for the given GHES version
	Constraint VersionConstraint
}

// VersionConstraint is a function that determines if a variant applies to a given version
type VersionConstraint func(v *version.Version) bool

// GHESVersionConstraint creates a constraint for comparing against a GHES version.
// Supported operators: "<", "<=", ">", ">=", "==", "!="
func GHESVersionConstraint(operator, versionStr string) VersionConstraint {
	targetVersion, err := version.NewVersion(versionStr)
	if err != nil {
		// If version parsing fails, return a constraint that never matches
		return func(v *version.Version) bool { return false }
	}

	return func(v *version.Version) bool {
		switch operator {
		case "<":
			return v.LessThan(targetVersion)
		case "<=":
			return v.LessThanOrEqual(targetVersion)
		case ">":
			return v.GreaterThan(targetVersion)
		case ">=":
			return v.GreaterThanOrEqual(targetVersion)
		case "==":
			return v.Equal(targetVersion)
		case "!=":
			return !v.Equal(targetVersion)
		default:
			return false
		}
	}
}

// GHESVersionAll creates a constraint that matches all GHES versions.
// Use this when a variant should apply to all GHES hosts regardless of version.
func GHESVersionAll() VersionConstraint {
	return func(v *version.Version) bool {
		return true
	}
}

// GHESVersionRange creates a constraint that matches versions within a range [min, max)
func GHESVersionRange(minVersion, maxVersion string) VersionConstraint {
	minV, minErr := version.NewVersion(minVersion)
	maxV, maxErr := version.NewVersion(maxVersion)

	return func(v *version.Version) bool {
		if minErr != nil || maxErr != nil {
			return false
		}
		return v.GreaterThanOrEqual(minV) && v.LessThan(maxV)
	}
}

// Command builds and returns the cobra.Command with version-routing RunE
func (vc *VersionedCommand) Command() *cobra.Command {
	if vc.Cmd == nil {
		panic("VersionedCommand.Cmd is required")
	}
	if vc.DefaultRunE == nil {
		panic("VersionedCommand.DefaultRunE is required")
	}

	// Replace the command's RunE with version-routing logic
	vc.Cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Resolve the target host
		host, err := vc.HostResolver()
		if err != nil {
			return err
		}

		// For github.com, always use the default implementation
		if !ghauth.IsEnterprise(host) {
			return vc.DefaultRunE(cmd, args)
		}

		// For GHES, resolve the version and find matching variant
		httpClient, err := vc.HttpClient()
		if err != nil {
			return err
		}

		ghesVersion, err := resolveGHESVersion(httpClient, host)
		if err != nil {
			// If we can't determine version, fall back to default
			// This maintains backwards compatibility
			return vc.DefaultRunE(cmd, args)
		}

		// Check each variant for a match
		for variantName, variant := range vc.Variants {
			if variant.Constraint(ghesVersion) {
				if variant.RunE == nil {
					return fmt.Errorf("variant %q for command %q has no RunE", variantName, vc.Name)
				}
				return variant.RunE(cmd, args)
			}
		}

		// No variant matched, use default
		return vc.DefaultRunE(cmd, args)
	}

	return vc.Cmd
}

// resolveGHESVersion queries the /meta endpoint to get the GHES version
func resolveGHESVersion(httpClient *http.Client, host string) (*version.Version, error) {
	// Import the resolver from featuredetection would create a cycle,
	// so we duplicate the minimal logic here
	url := fmt.Sprintf("https://%s/api/v3/meta", host)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get GHES version: %s", resp.Status)
	}

	var meta struct {
		InstalledVersion string `json:"installed_version"`
	}

	if err := decodeJSON(resp.Body, &meta); err != nil {
		return nil, err
	}

	return version.NewVersion(meta.InstalledVersion)
}

// decodeJSON decodes JSON from a reader into the target
func decodeJSON(r io.Reader, target interface{}) error {
	return json.NewDecoder(r).Decode(target)
}

// RepoHostResolver creates a standard HostResolver that uses a BaseRepo function.
// This is the common case where the host is derived from the resolved repository.
func RepoHostResolver(baseRepoFn func() (ghrepo.Interface, error)) func() (string, error) {
	return func() (string, error) {
		repo, err := baseRepoFn()
		if err != nil {
			return "", err
		}
		return repo.RepoHost(), nil
	}
}
