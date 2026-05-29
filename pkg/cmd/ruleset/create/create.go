package create

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
)

type CreateOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	BaseRepo   func() (ghrepo.Interface, error)

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
}

func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := CreateOptions{
		HttpClient: f.HttpClient,
		IO:         f.IOStreams,
	}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a ruleset",
		Long: heredoc.Doc(`
			Create a repository or organization ruleset.

			Use flags for common configurations, or --from-json for full control.
			PR rule shortcut flags (--required-approvals, etc.) are merged into a
			single pull_request rule.
		`),
		Example: heredoc.Doc(`
			# Protect the default branch with PR reviews
			$ gh ruleset create protect-main --include-refs "~DEFAULT_BRANCH" --required-approvals 2

			# Block force pushes and deletions
			$ gh ruleset create safety --include-refs "~ALL" --rule non_fast_forward --rule deletion

			# Full control via JSON
			$ gh ruleset create my-rules --from-json ruleset.json

			# Org-level ruleset
			$ gh ruleset create org-policy --org my-org --enforcement evaluate --required-approvals 1
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			opts.BaseRepo = f.BaseRepo

			if opts.FromJSON == "" {
				if len(args) == 0 {
					return cmdutil.FlagErrorf("name argument required (or use --from-json)")
				}
				opts.Name = args[0]
			} else if len(args) > 0 {
				opts.Name = args[0]
			}

			if opts.FromJSON == "" {
				if !slices.Contains(shared.ValidEnforcements, opts.Enforcement) {
					return cmdutil.FlagErrorf("invalid enforcement %q, must be one of: %s", opts.Enforcement, joinOr(shared.ValidEnforcements))
				}
				if !slices.Contains(shared.ValidTargets, opts.Target) {
					return cmdutil.FlagErrorf("invalid target %q, must be one of: %s", opts.Target, joinOr(shared.ValidTargets))
				}
			}

			if runF != nil {
				return runF(&opts)
			}
			return createRun(&opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Target, "target", "t", "branch", "Target type: branch, tag, push")
	cmd.Flags().StringVarP(&opts.Enforcement, "enforcement", "e", "active", "Enforcement level: active, evaluate, disabled")
	cmd.Flags().StringSliceVar(&opts.IncludeRefs, "include-refs", nil, "Ref patterns to include (e.g. ~DEFAULT_BRANCH, refs/heads/main)")
	cmd.Flags().StringSliceVar(&opts.ExcludeRefs, "exclude-refs", nil, "Ref patterns to exclude")
	cmd.Flags().StringSliceVar(&opts.BypassActors, "bypass-actor", nil, "Bypass actors in id:type:mode format (e.g. 123:Team:always)")
	cmd.Flags().StringSliceVar(&opts.Rules, "rule", nil, "Simple rule types (e.g. deletion, required_signatures, non_fast_forward)")
	cmd.Flags().StringSliceVar(&opts.AddRules, "add-rule", nil, "Raw JSON rule objects")
	cmd.Flags().StringVar(&opts.FromJSON, "from-json", "", "Path to JSON file with complete request body (use - for stdin)")
	cmd.Flags().StringVarP(&opts.Organization, "org", "o", "", "Organization that owns the ruleset")
	cmd.Flags().IntVar(&opts.RequiredApprovals, "required-approvals", 0, "Required number of PR approvals (creates a pull_request rule)")
	cmd.Flags().BoolVar(&opts.DismissStaleReviews, "dismiss-stale-reviews", false, "Dismiss stale PR reviews on push")
	cmd.Flags().BoolVar(&opts.RequireCodeOwnerReview, "require-code-owner-review", false, "Require code owner review")
	cmd.Flags().BoolVar(&opts.RequireLastPushApproval, "require-last-push-approval", false, "Require approval from someone other than the last pusher")
	cmd.Flags().BoolVar(&opts.RequiredReviewThreadResolution, "require-review-thread-resolution", false, "Require all review threads to be resolved")

	return cmd
}

func createRun(opts *CreateOptions) error {
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
		body, err = buildRequestBody(opts)
		if err != nil {
			return err
		}
	}

	var hostname, path, entityName string

	if opts.Organization != "" {
		hostname = hostForOrg(opts)
		entityName = opts.Organization
		path = fmt.Sprintf("orgs/%s/rulesets", opts.Organization)
	} else {
		baseRepo, err := opts.BaseRepo()
		if err != nil {
			return err
		}
		hostname = baseRepo.RepoHost()
		entityName = ghrepo.FullName(baseRepo)
		path = fmt.Sprintf("repos/%s/%s/rulesets", baseRepo.RepoOwner(), baseRepo.RepoName())
	}

	opts.IO.StartProgressIndicator()
	apiClient := api.NewClientFromHTTP(httpClient)
	var result shared.RulesetREST
	err = apiClient.REST(hostname, "POST", path, body, &result)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}

	if opts.IO.IsStdoutTTY() {
		cs := opts.IO.ColorScheme()
		fmt.Fprintf(opts.IO.Out, "%s Ruleset %q (ID: %d) created in %s\n", cs.SuccessIcon(), result.Name, result.Id, entityName)
	} else {
		fmt.Fprintf(opts.IO.Out, "%d\n", result.Id)
	}

	return nil
}

func buildRequestBody(opts *CreateOptions) (io.Reader, error) {
	req := shared.RulesetRequest{
		Name:        opts.Name,
		Target:      opts.Target,
		Enforcement: opts.Enforcement,
		Rules:       []shared.RuleRequest{},
	}

	// Conditions
	if len(opts.IncludeRefs) > 0 || len(opts.ExcludeRefs) > 0 {
		refName := map[string]interface{}{}
		if len(opts.IncludeRefs) > 0 {
			refName["include"] = opts.IncludeRefs
		} else {
			refName["include"] = []string{}
		}
		if len(opts.ExcludeRefs) > 0 {
			refName["exclude"] = opts.ExcludeRefs
		} else {
			refName["exclude"] = []string{}
		}
		req.Conditions = map[string]interface{}{
			"ref_name": refName,
		}
	}

	// Bypass actors
	for _, ba := range opts.BypassActors {
		actor, err := shared.ParseBypassActor(ba)
		if err != nil {
			return nil, err
		}
		req.BypassActors = append(req.BypassActors, actor)
	}

	// Simple rules
	for _, r := range opts.Rules {
		req.Rules = append(req.Rules, shared.RuleRequest{Type: r})
	}

	// PR rule shortcuts
	hasPRFlags := opts.RequiredApprovals > 0 || opts.DismissStaleReviews || opts.RequireCodeOwnerReview || opts.RequireLastPushApproval || opts.RequiredReviewThreadResolution
	if hasPRFlags {
		approvals := opts.RequiredApprovals
		if approvals == 0 {
			approvals = 1
		}
		prRule := shared.BuildPullRequestRule(approvals, opts.DismissStaleReviews, opts.RequireCodeOwnerReview, opts.RequireLastPushApproval, opts.RequiredReviewThreadResolution)
		req.Rules = append(req.Rules, prRule)
	}

	// Raw JSON rules
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

func readJSONInput(opts *CreateOptions) (io.Reader, error) {
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

func hostForOrg(opts *CreateOptions) string {
	if baseRepo, err := opts.BaseRepo(); err == nil {
		return baseRepo.RepoHost()
	}
	return "github.com"
}

func joinOr(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return fmt.Sprintf("%s", quoted)
}
