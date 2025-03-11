package azure

import (
	"os"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

type AzureDevOpsConfig struct {
	OrganizationURL     string
	PersonalAccessToken string
	Project             string
}

type AzureProvider struct {
	Tools  map[string]core.Tool
	config AzureDevOpsConfig
}

func NewAzureProvider() *AzureProvider {
	config := AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + os.Getenv("AZURE_DEVOPS_ORG"),
		PersonalAccessToken: os.Getenv("AZDO_PAT"),
		Project:             os.Getenv("AZURE_DEVOPS_PROJECT"),
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	return &AzureProvider{
		config: config,
		Tools: map[string]core.Tool{
			"work_item": NewWorkItemTool(conn, config),
			"wiki":      NewWikiTool(conn, config),
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
