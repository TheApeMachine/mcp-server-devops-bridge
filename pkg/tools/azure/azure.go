package azure

import (
	"log"
	"os"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/azure/tools"
)

type AzureDevOpsConfig struct {
	OrganizationURL     string
	PersonalAccessToken string
	Project             string
	Team                string
}

type AzureProvider struct {
	Tools  map[string]core.Tool
	config AzureDevOpsConfig
}

func NewAzureProvider() *AzureProvider {
	// Check if the required environment variables are set
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		log.Println("Warning: Azure DevOps environment variables not set correctly")
		log.Println("Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM")
		return &AzureProvider{
			Tools: make(map[string]core.Tool),
		}
	}

	config := AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)
	provider := &AzureProvider{
		config: config,
		Tools:  make(map[string]core.Tool),
	}

	// Register individual tools
	toolsConfig := tools.AzureDevOpsConfig{
		OrganizationURL:     config.OrganizationURL,
		PersonalAccessToken: config.PersonalAccessToken,
		Project:             config.Project,
		Team:                config.Team,
	}

	/*
		- `get_sprints`: Get all sprints in Azure DevOps, useful for the model to understand which sprints exist, especially when creating new sprints.
		- `create_sprint`: Create a new sprint in Azure DevOps.
		- `sprint_items`: Get the items in a sprint in Azure DevOps.
		- `sprint_overview`: Get the overview of a sprint in Azure DevOps.
		- `get_work_items`: Get the details of a work item in Azure DevOps.
		- `create_work_items`: Create a new work item in Azure DevOps.
		- `update_work_items`: Update a work item in Azure DevOps. This should be capable of dealing with the full range of work item fields, including assignment, status, custom fields, sprint, relationships, comments, etc.
		- `search_work_items`: Search for work items in Azure DevOps by keywords, abstracting away the WIQL query.
		- `execute_wiql`: Execute a WIQL query on Azure DevOps, returning the results.
	*/

	// Initialize new modular tools
	provider.registerTool(tools.NewAzureGetSprintsTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureCreateSprintTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureSprintItemsTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureSprintOverviewTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureGetWorkItemsTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureCreateWorkItemsTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureUpdateWorkItemsTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureExecuteWiqlTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureSearchWorkItemsTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureEnrichWorkItemTool(conn, toolsConfig))
	provider.registerTool(tools.NewAzureGetGitHubFileContentTool())

	wikiTool := NewWikiTool(conn, config)
	if wikiTool != nil {
		provider.Tools["wiki"] = wikiTool
	} else {
		log.Println("Warning: Failed to initialize Wiki tool")
	}

	return provider
}

// Helper method to register a tool with error handling
func (provider *AzureProvider) registerTool(tool core.Tool) {
	if tool == nil {
		return
	}

	toolName := tool.Handle().Name
	provider.Tools[toolName] = tool
	log.Printf("Registered Azure DevOps tool: %s", toolName)
}

func stringPtr(s string) *string {
	return &s
}
