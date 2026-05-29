package update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/ruleset/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type UpdateOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	BaseRepo   func() (ghrepo.Interface, error)

	ID           string
	Name         string
	Target       string
	Enforcement  string
	IncludeRefs  []string
	ExcludeRefs  []string
	BypassActors []string
	Rules        []string
	AddRules     []string
	FromJSON     string
	Organization string

	RequiredApprovals              int
	DismissStaleReviews            bool
	RequireCodeOwnerReview         bool
	RequireLastPushApproval        bool
	RequiredReviewThreadResolution bool

	// Track which flags were explicitly set
	flagsChanged map[string]bool
}

func NewCmdUpdate(f *cmdutil.Factory, runF func(*UpdateOptions) error) *cobra.Command {
	opts := UpdateOptions{
		HttpClient:   f.HttpClient,
		IO:           f.IOStreams,
		flagsChanged: make(map[string]bool),
	}

	cmd := &cobra.Command{
		Use:   "update <ruleset-id>",
		Short: "Update a ruleset",
		Long: heredoc.Doc(`
			Update a repository or organization ruleset.

			Fetches the current ruleset, applies your changes, and saves.
			Use --from-json for a full replacement.
		`),
		Example: heredoc.Doc(`
			# Change enforcement level
			$ gh ruleset update 42 --enforcement disabled

			# Rename a ruleset
			$ gh ruleset update 42 --name new-name

			# Full replacement from JSON
			$ gh ruleset update 42 --from-json updated.json
		`),
		Args: cmdutil.ExactArgs(1, "cannot update ruleset: ID argument required"),
		RunE: func(c *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo
			opts.ID = args[0]

			// Track which flags were explicitly set
			c.Flags().Visit(func(f *pflag.Flag) {
				opts.flagsChanged[f.Name] = true
			})

			if opts.FromJSON == "" {
				if e, ok := opts.flagsChanged["enforcement"]; ok && e {
					if !slices.Contains(shared.ValidEnforcements, opts.Enforcement) {
						return cmdutil.FlagErrorf("invalid enforcement %q", opts.Enforcement)
					}
				}
				if t, ok := opts.flagsChanged["target"]; ok && t {
					if !slices.Contains(shared.ValidTargets, opts.Target) {
						return cmdutil.FlagErrorf("invalid target %q", opts.Target)
					}
				}
			}

			if runF != nil {
				return runF(&opts)
			}
			return updateRun(&opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "New name for the ruleset")
	cmd.Flags().StringVarP(&opts.Target, "target", "t", "", "Target type: branch, tag, push")
	cmd.Flags().StringVarP(&opts.Enforcement, "enforcement", "e", "", "Enforcement level: active, evaluate, disabled")
	cmd.Flags().StringSliceVar(&opts.IncludeRefs, "include-refs", nil, "Ref patterns to include")
	cmd.Flags().StringSliceVar(&opts.ExcludeRefs, "exclude-refs", nil, "Ref patterns to exclude")
	cmd.Flags().StringSliceVar(&opts.BypassActors, "bypass-actor", nil, "Bypass actors in id:type:mode format")
	cmd.Flags().StringSliceVar(&opts.Rules, "rule", nil, "Simple rule types (replaces existing rules)")
	cmd.Flags().StringSliceVar(&opts.AddRules, "add-rule", nil, "Raw JSON rule objects (appended to existing)")
	cmd.Flags().StringVar(&opts.FromJSON, "from-json", "", "Path to JSON file with complete request body (use - for stdin)")
	cmd.Flags().StringVarP(&opts.Organization, "org", "o", "", "Organization that owns the ruleset")
	cmd.Flags().IntVar(&opts.RequiredApprovals, "required-approvals", 0, "Required number of PR approvals")
	cmd.Flags().BoolVar(&opts.DismissStaleReviews, "dismiss-stale-reviews", false, "Dismiss stale PR reviews on push")
	cmd.Flags().BoolVar(&opts.RequireCodeOwnerReview, "require-code-owner-review", false, "Require code owner review")
	cmd.Flags().BoolVar(&opts.RequireLastPushApproval, "require-last-push-approval", false, "Require approval from someone other than the last pusher")
	cmd.Flags().BoolVar(&opts.RequiredReviewThreadResolution, "require-review-thread-resolution", false, "Require all review threads to be resolved")

	return cmd
}

func updateRun(opts *UpdateOptions) error {
	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}

	var body io.Reader

	if opts.FromJSON != "" {
		body, err = readJSONInput(opts)
		if err != nil {
			return err
		}
	} else {
		body, err = buildMergedBody(httpClient, opts)
		if err != nil {
			return err
		}
	}

	var hostname, path, entityName string

	if opts.Organization != "" {
		hostname = hostForOrg(opts)
		entityName = opts.Organization
		path = fmt.Sprintf("orgs/%s/rulesets/%s", opts.Organization, opts.ID)
	} else {
		baseRepo, err := opts.BaseRepo()
		if err != nil {
			return err
		}
		hostname = baseRepo.RepoHost()
		entityName = ghrepo.FullName(baseRepo)
		path = fmt.Sprintf("repos/%s/%s/rulesets/%s", baseRepo.RepoOwner(), baseRepo.RepoName(), opts.ID)
	}

	opts.IO.StartProgressIndicator()
	apiClient := api.NewClientFromHTTP(httpClient)
	var result shared.RulesetREST
	err = apiClient.REST(hostname, "PUT", path, body, &result)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "%s Ruleset %q (ID: %d) updated in %s\n", cs.SuccessIcon(), result.Name, result.Id, entityName)
	}

	return nil
}

func buildMergedBody(httpClient *http.Client, opts *UpdateOptions) (io.Reader, error) {
	// Fetch current ruleset
	var current *shared.RulesetREST
	var err error

	if opts.Organization != "" {
		hostname := hostForOrg(opts)
		current, err = shared.GetOrgRuleset(httpClient, opts.Organization, opts.ID, hostname)
	} else {
		baseRepo, err2 := opts.BaseRepo()
		if err2 != nil {
			return nil, err2
		}
		current, err = shared.GetRepoRuleset(httpClient, baseRepo, opts.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current ruleset: %w", err)
	}

	req := shared.RulesetRESTToRequest(current)

	// Apply overrides for explicitly set flags
	if opts.flagsChanged["name"] {
		req.Name = opts.Name
	}
	if opts.flagsChanged["target"] {
		req.Target = opts.Target
	}
	if opts.flagsChanged["enforcement"] {
		req.Enforcement = opts.Enforcement
	}

	if opts.flagsChanged["include-refs"] || opts.flagsChanged["exclude-refs"] {
		refName := map[string]interface{}{}
		if opts.flagsChanged["include-refs"] {
			refName["include"] = opts.IncludeRefs
		} else if existing, ok := req.Conditions["ref_name"]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				refName["include"] = m["include"]
			}
		}
		if opts.flagsChanged["exclude-refs"] {
			refName["exclude"] = opts.ExcludeRefs
		} else if existing, ok := req.Conditions["ref_name"]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				refName["exclude"] = m["exclude"]
			}
		}
		if req.Conditions == nil {
			req.Conditions = make(map[string]interface{})
		}
		req.Conditions["ref_name"] = refName
	}

	if opts.flagsChanged["bypass-actor"] {
		req.BypassActors = nil
		for _, ba := range opts.BypassActors {
			actor, err := shared.ParseBypassActor(ba)
			if err != nil {
				return nil, err
			}
			req.BypassActors = append(req.BypassActors, actor)
		}
	}

	if opts.flagsChanged["rule"] {
		req.Rules = nil
		for _, r := range opts.Rules {
			req.Rules = append(req.Rules, shared.RuleRequest{Type: r})
		}
	}

	hasPRFlags := opts.flagsChanged["required-approvals"] || opts.flagsChanged["dismiss-stale-reviews"] || opts.flagsChanged["require-code-owner-review"] || opts.flagsChanged["require-last-push-approval"] || opts.flagsChanged["require-review-thread-resolution"]
	if hasPRFlags {
		approvals := opts.RequiredApprovals
		if approvals == 0 {
			approvals = 1
		}
		prRule := shared.BuildPullRequestRule(approvals, opts.DismissStaleReviews, opts.RequireCodeOwnerReview, opts.RequireLastPushApproval, opts.RequiredReviewThreadResolution)

		// Replace existing pull_request rule or append
		replaced := false
		for i, r := range req.Rules {
			if r.Type == "pull_request" {
				req.Rules[i] = prRule
				replaced = true
				break
			}
		}
		if !replaced {
			req.Rules = append(req.Rules, prRule)
		}
	}

	for _, raw := range opts.AddRules {
		var rule shared.RuleRequest
		if err := json.Unmarshal([]byte(raw), &rule); err != nil {
			return nil, fmt.Errorf("invalid --add-rule JSON: %w", err)
		}
		req.Rules = append(req.Rules, rule)
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func readJSONInput(opts *UpdateOptions) (io.Reader, error) {
	if opts.FromJSON == "-" {
		data, err := io.ReadAll(opts.IO.In)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		return bytes.NewReader(data), nil
	}

	data, err := os.ReadFile(opts.FromJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", opts.FromJSON, err)
	}
	return bytes.NewReader(data), nil
}

func hostForOrg(opts *UpdateOptions) string {
	if baseRepo, err := opts.BaseRepo(); err == nil {
		return baseRepo.RepoHost()
	}
	return "github.com"
}
