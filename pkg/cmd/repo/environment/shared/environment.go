package shared

import "github.com/cli/cli/v2/pkg/cmdutil"

type Environment struct {
	ID                     int                     `json:"id"`
	Name                   string                  `json:"name"`
	URL                    string                  `json:"url"`
	HTMLURL                string                  `json:"html_url"`
	CreatedAt              string                  `json:"created_at"`
	UpdatedAt              string                  `json:"updated_at"`
	CanAdminsBypass        bool                    `json:"can_admins_bypass"`
	ProtectionRules        []ProtectionRule        `json:"protection_rules"`
	DeploymentBranchPolicy *DeploymentBranchPolicy `json:"deployment_branch_policy"`
}

type ProtectionRule struct {
	ID        int        `json:"id"`
	Type      string     `json:"type"`
	WaitTimer int        `json:"wait_timer"`
	Reviewers []Reviewer `json:"reviewers"`
}

type Reviewer struct {
	Type     string       `json:"type"`
	Reviewer ReviewerInfo `json:"reviewer"`
}

type ReviewerInfo struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Slug  string `json:"slug"`
}

type DeploymentBranchPolicy struct {
	ProtectedBranches    bool `json:"protected_branches"`
	CustomBranchPolicies bool `json:"custom_branch_policies"`
}

type EnvironmentListResponse struct {
	TotalCount   int           `json:"total_count"`
	Environments []Environment `json:"environments"`
}

var EnvironmentFields = []string{
	"id",
	"name",
	"url",
	"htmlUrl",
	"createdAt",
	"updatedAt",
	"canAdminsBypass",
	"protectionRules",
	"deploymentBranchPolicy",
}

func (e *Environment) ExportData(fields []string) map[string]interface{} {
	return cmdutil.StructExportData(e, fields)
}
