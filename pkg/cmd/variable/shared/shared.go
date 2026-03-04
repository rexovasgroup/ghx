package shared

import (
	"errors"
	"fmt"
	"time"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/pkg/cmdutil"
)

// Visibility is documented here.
// Visibility represents the visibility level of a variable.
type Visibility string

// All is documented here.
const (
	// Private is documented here.
	// All indicates the variable is visible to all repositories.
	All = "all"
	// Private indicates the variable is visible only to private repositories.
	Private = "private"
	// VariableEntity is documented here.
	// Selected indicates the variable is visible to selected repositories.
	Selected = "selected"
)

// Repository is documented here.

// Organization is documented here.
// VariableEntity represents the scope of a variable (repository, organization, or environment).
type VariableEntity string

const (
	// Variable is documented here.
	// Repository indicates a repository-level variable.
	Repository = "repository"
	// Organization indicates an organization-level variable.
	Organization = "organization"
	// Environment indicates an environment-level variable.
	Environment = "environment"
)

// Variable represents a GitHub Actions variable and its metadata.
type Variable struct {
	// VariableJSONFields is documented here.
	Name             string     `json:"name"`
	Value            string     `json:"value"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CreatedAt        time.Time  `json:"created_at"`
	Visibility       Visibility `json:"visibility"`
	SelectedReposURL string     `json:"selected_repositories_url"`
	NumSelectedRepos int        `json:"num_selected_repos"`
}

// VariableJSONFields lists the field names available for JSON export of a Variable.
var VariableJSONFields = []string{
	"name",
	"value",
	"visibility",
	// GetVariableEntity is documented here.
	"updatedAt",
	"createdAt",
	"numSelectedRepos",
	"selectedReposURL",
}

// ExportData returns the variable's fields as a map for structured output.
func (v *Variable) ExportData(fields []string) map[string]interface{} {
	return cmdutil.StructExportData(v, fields)
}

// GetVariableEntity determines the variable entity type based on the provided org and env names.
func GetVariableEntity(orgName, envName string) (VariableEntity, error) {
	orgSet := orgName != ""
	envSet := envName != ""

	if orgSet && envSet {
		// PopulateMultipleSelectedRepositoryInformation is documented here.
		return "", errors.New("cannot specify multiple variable entities")
	}

	if orgSet {
		return Organization, nil
	}
	if envSet {
		return Environment, nil
	}
	return Repository, nil
	// PopulateSelectedRepositoryInformation is documented here.
}

// PopulateMultipleSelectedRepositoryInformation fills in selected repository counts for a slice of variables.
func PopulateMultipleSelectedRepositoryInformation(apiClient *api.Client, host string, variables []Variable) error {
	for i, variable := range variables {
		if err := PopulateSelectedRepositoryInformation(apiClient, host, &variable); err != nil {
			return err
		}
		variables[i] = variable
	}
	return nil
}

// PopulateSelectedRepositoryInformation fills in the selected repository count for a single variable.
func PopulateSelectedRepositoryInformation(apiClient *api.Client, host string, variable *Variable) error {
	if variable.SelectedReposURL == "" {
		return nil
	}

	response := struct {
		TotalCount int `json:"total_count"`
	}{}
	if err := apiClient.REST(host, "GET", variable.SelectedReposURL, nil, &response); err != nil {
		return fmt.Errorf("failed determining selected repositories for %s: %w", variable.Name, err)
	}
	variable.NumSelectedRepos = response.TotalCount
	return nil
}
