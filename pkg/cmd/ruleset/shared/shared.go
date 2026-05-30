package shared

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmdutil"
)

// Request body types for POST/PUT endpoints

type RulesetRequest struct {
	Name         string                 `json:"name"`
	Target       string                 `json:"target"`
	Enforcement  string                 `json:"enforcement"`
	Conditions   map[string]interface{} `json:"conditions,omitempty"`
	BypassActors []BypassActor          `json:"bypass_actors,omitempty"`
	Rules        []RuleRequest          `json:"rules"`
}

type BypassActor struct {
	ActorID    *int   `json:"actor_id"`
	ActorType  string `json:"actor_type"`
	BypassMode string `json:"bypass_mode"`
}

type RuleRequest struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

var ValidEnforcements = []string{"active", "evaluate", "disabled"}
var ValidTargets = []string{"branch", "tag", "push"}

func ParseBypassActor(s string) (BypassActor, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return BypassActor{}, fmt.Errorf("invalid bypass-actor format %q, expected id:type:mode (e.g. 123:Team:always)", s)
	}

	var actorID *int
	if parts[0] != "" && parts[0] != "null" {
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			return BypassActor{}, fmt.Errorf("invalid actor_id %q: must be an integer", parts[0])
		}
		actorID = &id
	}

	return BypassActor{
		ActorID:    actorID,
		ActorType:  parts[1],
		BypassMode: parts[2],
	}, nil
}

func BuildPullRequestRule(approvals int, dismissStale, codeOwner, lastPush, threadResolution bool, allowedMergeMethods []string) RuleRequest {
	params := map[string]interface{}{
		"required_approving_review_count":   approvals,
		"dismiss_stale_reviews_on_push":     dismissStale,
		"require_code_owner_review":         codeOwner,
		"require_last_push_approval":        lastPush,
		"required_review_thread_resolution": threadResolution,
	}
	if len(allowedMergeMethods) > 0 {
		params["allowed_merge_methods"] = allowedMergeMethods
	}
	return RuleRequest{
		Type:       "pull_request",
		Parameters: params,
	}
}

func RulesetRESTToRequest(rs *RulesetREST) *RulesetRequest {
	req := &RulesetRequest{
		Name:        rs.Name,
		Target:      rs.Target,
		Enforcement: rs.Enforcement,
		Conditions:  make(map[string]interface{}),
		Rules:       make([]RuleRequest, 0, len(rs.Rules)),
	}

	for k, v := range rs.Conditions {
		req.Conditions[k] = v
	}

	for _, ba := range rs.BypassActors {
		id := ba.ActorId
		req.BypassActors = append(req.BypassActors, BypassActor{
			ActorID:    &id,
			ActorType:  ba.ActorType,
			BypassMode: ba.BypassMode,
		})
	}

	for _, rule := range rs.Rules {
		req.Rules = append(req.Rules, RuleRequest{
			Type:       rule.Type,
			Parameters: rule.Parameters,
		})
	}

	return req
}

type RulesetGraphQL struct {
	DatabaseId  int
	Name        string
	Target      string
	Enforcement string
	Source      struct {
		TypeName string `json:"__typename"`
		Owner    string
	}
	Rules struct {
		TotalCount int
	}
}

type RulesetREST struct {
	Id                   int
	Name                 string
	Target               string
	Enforcement          string
	CurrentUserCanBypass string `json:"current_user_can_bypass"`
	BypassActors         []struct {
		ActorId    int    `json:"actor_id"`
		ActorType  string `json:"actor_type"`
		BypassMode string `json:"bypass_mode"`
	} `json:"bypass_actors"`
	Conditions map[string]map[string]interface{}
	SourceType string `json:"source_type"`
	Source     string
	Rules      []RulesetRule
	Links      struct {
		Html struct {
			Href string
		}
	} `json:"_links"`
}

type RulesetRule struct {
	Type              string
	Parameters        map[string]interface{}
	RulesetSourceType string `json:"ruleset_source_type"`
	RulesetSource     string `json:"ruleset_source"`
	RulesetId         int    `json:"ruleset_id"`
}

// Returns the source of the ruleset in the format "owner/name (repo)" or "owner (org)"
func RulesetSource(rs RulesetGraphQL) string {
	var level string
	if rs.Source.TypeName == "Repository" {
		level = "repo"
	} else if rs.Source.TypeName == "Organization" {
		level = "org"
	} else {
		level = "unknown"
	}

	return fmt.Sprintf("%s (%s)", rs.Source.Owner, level)
}

func ParseRulesForDisplay(rules []RulesetRule) string {
	var display strings.Builder

	// sort keys for consistent responses
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Type < rules[j].Type
	})

	for _, rule := range rules {
		display.WriteString(fmt.Sprintf("- %s", rule.Type))

		if len(rule.Parameters) > 0 {
			display.WriteString(": ")

			// sort these keys too for consistency
			params := make([]string, 0, len(rule.Parameters))
			for p := range rule.Parameters {
				params = append(params, p)
			}
			sort.Strings(params)

			for _, n := range params {
				display.WriteString(fmt.Sprintf("[%s: %v] ", n, rule.Parameters[n]))
			}
		}

		// ruleset source info is only returned from the "get rules for a branch" endpoint
		if rule.RulesetSource != "" {
			display.WriteString(
				fmt.Sprintf(
					"\n  (configured in ruleset %d from %s %s)\n",
					rule.RulesetId,
					strings.ToLower(rule.RulesetSourceType),
					rule.RulesetSource,
				),
			)
		}

		display.WriteString("\n")
	}

	return display.String()
}

func NoRulesetsFoundError(orgOption string, repoI ghrepo.Interface, includeParents bool) error {
	entityName := EntityName(orgOption, repoI)
	parentsMsg := ""
	if includeParents {
		parentsMsg = " or its parents"
	}
	return cmdutil.NewNoResultsError(fmt.Sprintf("no rulesets found in %s%s", entityName, parentsMsg))
}

func EntityName(orgOption string, repoI ghrepo.Interface) string {
	if orgOption != "" {
		return orgOption
	}
	return ghrepo.FullName(repoI)
}
