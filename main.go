package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/wiki"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// AzureDevOpsConfig holds the configuration for Azure DevOps connection
type AzureDevOpsConfig struct {
	OrganizationURL     string
	PersonalAccessToken string
	Project             string
}

// Global clients and config
var (
	connection     *azuredevops.Connection
	workItemClient workitemtracking.Client
	wikiClient     wiki.Client
	coreClient     core.Client
	config         AzureDevOpsConfig
	intConfig      IntegrationsConfig
)

func main() {
	// Load configuration from environment variables
	config = AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + os.Getenv("AZURE_DEVOPS_ORG"),
		PersonalAccessToken: os.Getenv("AZDO_PAT"),
		Project:             os.Getenv("AZURE_DEVOPS_PROJECT"),
	}

	intConfig = IntegrationsConfig{
		GithubToken: os.Getenv("GITHUB_PAT"),
		SlackToken:  os.Getenv("SLACK_BOT_TOKEN"),
	}

	// Validate configuration
	if config.OrganizationURL == "" || config.PersonalAccessToken == "" || config.Project == "" {
		log.Fatal("Missing required environment variables: AZURE_DEVOPS_ORG_URL, AZURE_DEVOPS_PAT, AZURE_DEVOPS_PROJECT")
	}

	// Initialize Azure DevOps clients
	if err := initializeClients(config); err != nil {
		log.Fatalf("Failed to initialize Azure DevOps clients: %v", err)
	}

	// Initialize integration clients
	if err := initializeIntegrationClients(intConfig); err != nil {
		log.Printf("Warning: Failed to initialize integration clients: %v", err)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"Azure DevOps MCP Server",
		"1.0.0",
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
	)

	// Configure custom error handling
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(&logWriter{})

	// Add Work Item tools
	addWorkItemTools(s)

	// Add Wiki tools
	addWikiTools(s)

	// Add Integration tools
	addIntegrationTools(s)

	// Start the server
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtr(s string) *string {
	return &s
}

// Initialize Azure DevOps clients
func initializeClients(config AzureDevOpsConfig) error {
	connection = azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	ctx := context.Background()

	var err error

	// Initialize Work Item Tracking client
	workItemClient, err = workitemtracking.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create work item client: %v", err)
	}

	// Initialize Wiki client
	wikiClient, err = wiki.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create wiki client: %v", err)
	}

	// Initialize Core client
	coreClient, err = core.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create core client: %v", err)
	}

	return nil
}

// Handler for getting detailed work item information
func handleGetWorkItemDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsStr := request.Params.Arguments["ids"].(string)
	idStrs := strings.Split(idsStr, ",")

	var ids []int
	for _, idStr := range idStrs {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid ID format: %s", idStr)), nil
		}
		ids = append(ids, id)
	}

	workItems, err := workItemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.All,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work items: %v", err)), nil
	}

	var results []string
	for _, item := range *workItems {
		fields := *item.Fields
		title, _ := fields["System.Title"].(string)
		description, _ := fields["System.Description"].(string)
		state, _ := fields["System.State"].(string)

		result := fmt.Sprintf("ID: %d\nTitle: %s\nState: %s\nDescription: %s\n---\n",
			*item.Id, title, state, description)
		results = append(results, result)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for managing work item relationships
func handleManageWorkItemRelations(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sourceID := int(request.Params.Arguments["source_id"].(float64))
	targetID := int(request.Params.Arguments["target_id"].(float64))
	relationType, ok := request.Params.Arguments["relation_type"].(string)
	if !ok {
		return mcp.NewToolResultError("Invalid relation_type"), nil
	}
	operation := request.Params.Arguments["operation"].(string)

	// Map relation types to Azure DevOps relation types
	relationTypeMap := map[string]string{
		"parent":  "System.LinkTypes.Hierarchy-Reverse",
		"child":   "System.LinkTypes.Hierarchy-Forward",
		"related": "System.LinkTypes.Related",
	}

	azureRelationType := relationTypeMap[relationType]

	var ops []webapi.JsonPatchOperation
	if operation == "add" {
		ops = []webapi.JsonPatchOperation{
			{
				Op:   &webapi.OperationValues.Add,
				Path: stringPtr("/relations/-"),
				Value: map[string]interface{}{
					"rel": azureRelationType,
					"url": fmt.Sprintf("%s/_apis/wit/workItems/%d", config.OrganizationURL, targetID),
					"attributes": map[string]interface{}{
						"comment": "Added via MCP",
					},
				},
			},
		}
	} else {
		// For remove, we need to first get the work item to find the relation index
		workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
			Id:      &sourceID,
			Project: &config.Project,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
		}

		if workItem.Relations == nil {
			return mcp.NewToolResultError("Work item has no relations"), nil
		}

		for i, relation := range *workItem.Relations {
			if *relation.Rel == azureRelationType {
				targetUrl := fmt.Sprintf("%s/_apis/wit/workItems/%d", config.OrganizationURL, targetID)
				if *relation.Url == targetUrl {
					ops = []webapi.JsonPatchOperation{
						{
							Op:   &webapi.OperationValues.Remove,
							Path: stringPtr(fmt.Sprintf("/relations/%d", i)),
						},
					}
					break
				}
			}
		}

		if len(ops) == 0 {
			return mcp.NewToolResultError("Specified relation not found"), nil
		}
	}

	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:       &sourceID,
		Project:  &config.Project,
		Document: &ops,
	}

	_, err := workItemClient.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update work item relations: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully %sd %s relationship", operation, relationType)), nil
}

// Handler for getting related work items
func handleGetRelatedWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))
	relationType := request.Params.Arguments["relation_type"].(string)

	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
	}

	if workItem.Relations == nil {
		return mcp.NewToolResultText("No related items found"), nil
	}

	relationTypeMap := map[string]string{
		"parent":   "System.LinkTypes.Hierarchy-Reverse",
		"children": "System.LinkTypes.Hierarchy-Forward",
		"related":  "System.LinkTypes.Related",
	}

	// Debug information
	var debugInfo []string
	debugInfo = append(debugInfo, fmt.Sprintf("Looking for relation type: %s (mapped to: %s)",
		relationType, relationTypeMap[relationType]))

	var relatedIds []int
	for _, relation := range *workItem.Relations {
		debugInfo = append(debugInfo, fmt.Sprintf("Found relation of type: %s", *relation.Rel))

		if relationType == "all" || *relation.Rel == relationTypeMap[relationType] {
			parts := strings.Split(*relation.Url, "/")
			if relatedID, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				relatedIds = append(relatedIds, relatedID)
			}
		}
	}

	if len(relatedIds) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Debug info:\n%s\n\nNo matching related items found",
			strings.Join(debugInfo, "\n"))), nil
	}

	// Get details of related items
	relatedItems, err := workItemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &relatedIds,
		Project: &config.Project,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get related items: %v", err)), nil
	}

	var results []string
	for _, item := range *relatedItems {
		fields := *item.Fields
		title, _ := fields["System.Title"].(string)
		result := fmt.Sprintf("ID: %d, Title: %s", *item.Id, title)
		results = append(results, result)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for adding a comment to a work item
func handleAddWorkItemComment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))
	text := request.Params.Arguments["text"].(string)

	// Add comment as a discussion by updating the Discussion field
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Add,
				Path:  stringPtr("/fields/System.History"),
				Value: text,
			},
		},
	}

	workItem, err := workItemClient.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add comment: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Added comment to work item #%d", *workItem.Id)), nil
}

// Handler for getting work item comments
func handleGetWorkItemComments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))

	comments, err := workItemClient.GetComments(ctx, workitemtracking.GetCommentsArgs{
		Project:    &config.Project,
		WorkItemId: &id,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get comments: %v", err)), nil
	}

	var results []string
	for _, comment := range *comments.Comments {
		results = append(results, fmt.Sprintf("Comment by %s at %s:\n%s\n---",
			*comment.CreatedBy.DisplayName,
			comment.CreatedDate.String(),
			*comment.Text))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for getting work item fields
func handleGetWorkItemFields(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["work_item_id"].(float64))

	// Get the work item's details
	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item details: %v", err)), nil
	}

	// Extract and format field information
	var results []string
	fieldName, hasFieldFilter := request.Params.Arguments["field_name"].(string)

	for fieldRef, value := range *workItem.Fields {
		if hasFieldFilter && !strings.Contains(strings.ToLower(fieldRef), strings.ToLower(fieldName)) {
			continue
		}

		results = append(results, fmt.Sprintf("Field: %s\nValue: %v\nType: %T\n---",
			fieldRef,
			value,
			value))
	}

	if len(results) == 0 {
		if hasFieldFilter {
			return mcp.NewToolResultText(fmt.Sprintf("No fields found matching: %s", fieldName)), nil
		}
		return mcp.NewToolResultText("No fields found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for batch creating work items
func handleBatchCreateWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	itemsJSON := request.Params.Arguments["items"].(string)
	var items []struct {
		Type        string `json:"type"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority,omitempty"`
	}

	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid JSON format: %v", err)), nil
	}

	var results []string
	for _, item := range items {
		createArgs := workitemtracking.CreateWorkItemArgs{
			Type:    &item.Type,
			Project: &config.Project,
			Document: &[]webapi.JsonPatchOperation{
				{
					Op:    &webapi.OperationValues.Add,
					Path:  stringPtr("/fields/System.Title"),
					Value: item.Title,
				},
				{
					Op:    &webapi.OperationValues.Add,
					Path:  stringPtr("/fields/System.Description"),
					Value: item.Description,
				},
			},
		}

		if item.Priority != "" {
			doc := append(*createArgs.Document, webapi.JsonPatchOperation{
				Op:    &webapi.OperationValues.Add,
				Path:  stringPtr("/fields/Microsoft.VSTS.Common.Priority"),
				Value: item.Priority,
			})
			createArgs.Document = &doc
		}

		workItem, err := workItemClient.CreateWorkItem(ctx, createArgs)
		if err != nil {
			results = append(results, fmt.Sprintf("Failed to create '%s': %v", item.Title, err))
			continue
		}
		results = append(results, fmt.Sprintf("Created work item #%d: %s", *workItem.Id, item.Title))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for batch updating work items
func handleBatchUpdateWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	updatesJSON := request.Params.Arguments["updates"].(string)
	var updates []struct {
		ID    int    `json:"id"`
		Field string `json:"field"`
		Value string `json:"value"`
	}

	if err := json.Unmarshal([]byte(updatesJSON), &updates); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid JSON format: %v", err)), nil
	}

	// Map field names to their System.* equivalents
	fieldMap := map[string]string{
		"Title":       "System.Title",
		"Description": "System.Description",
		"State":       "System.State",
		"Priority":    "Microsoft.VSTS.Common.Priority",
	}

	var results []string
	for _, update := range updates {
		systemField, ok := fieldMap[update.Field]
		if !ok {
			results = append(results, fmt.Sprintf("Invalid field for #%d: %s", update.ID, update.Field))
			continue
		}

		updateArgs := workitemtracking.UpdateWorkItemArgs{
			Id:      &update.ID,
			Project: &config.Project,
			Document: &[]webapi.JsonPatchOperation{
				{
					Op:    &webapi.OperationValues.Replace,
					Path:  stringPtr("/fields/" + systemField),
					Value: update.Value,
				},
			},
		}

		workItem, err := workItemClient.UpdateWorkItem(ctx, updateArgs)
		if err != nil {
			results = append(results, fmt.Sprintf("Failed to update #%d: %v", update.ID, err))
			continue
		}
		results = append(results, fmt.Sprintf("Updated work item #%d", *workItem.Id))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for managing work item tags
func handleManageWorkItemTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))
	operation := request.Params.Arguments["operation"].(string)
	tagsStr := request.Params.Arguments["tags"].(string)
	tags := strings.Split(tagsStr, ",")

	// Get current work item to get existing tags
	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
	}

	fields := *workItem.Fields
	var currentTags []string
	if tags, ok := fields["System.Tags"].(string); ok && tags != "" {
		currentTags = strings.Split(tags, "; ")
	}

	var newTags []string
	switch operation {
	case "add":
		// Add new tags while avoiding duplicates
		tagMap := make(map[string]bool)
		for _, tag := range currentTags {
			tagMap[strings.TrimSpace(tag)] = true
		}
		for _, tag := range tags {
			tagMap[strings.TrimSpace(tag)] = true
		}
		for tag := range tagMap {
			newTags = append(newTags, tag)
		}
	case "remove":
		// Remove specified tags
		tagMap := make(map[string]bool)
		for _, tag := range tags {
			tagMap[strings.TrimSpace(tag)] = true
		}
		for _, tag := range currentTags {
			if !tagMap[strings.TrimSpace(tag)] {
				newTags = append(newTags, tag)
			}
		}
	}

	// Update work item with new tags
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Replace,
				Path:  stringPtr("/fields/System.Tags"),
				Value: strings.Join(newTags, "; "),
			},
		},
	}

	_, err = workItemClient.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update tags: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully %sd tags for work item #%d", operation, id)), nil
}

// Handler for getting work item tags
func handleGetWorkItemTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))

	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
	}

	fields := *workItem.Fields
	if tags, ok := fields["System.Tags"].(string); ok && tags != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Tags for work item #%d:\n%s", id, tags)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("No tags found for work item #%d", id)), nil
}

// Handler for getting work item templates
func handleGetWorkItemTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workItemType := request.Params.Arguments["type"].(string)

	templates, err := workItemClient.GetTemplates(ctx, workitemtracking.GetTemplatesArgs{
		Project:          &config.Project,
		Team:             nil, // Get templates for entire project
		Workitemtypename: &workItemType,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get templates: %v", err)), nil
	}

	var results []string
	for _, template := range *templates {
		results = append(results, fmt.Sprintf("Template ID: %s\nName: %s\nDescription: %s\n---",
			*template.Id,
			*template.Name,
			*template.Description))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No templates found for type: %s", workItemType)), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for creating work item from template
func handleCreateFromTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templateID := request.Params.Arguments["template_id"].(string)
	fieldValuesJSON := request.Params.Arguments["field_values"].(string)

	var fieldValues map[string]interface{}
	if err := json.Unmarshal([]byte(fieldValuesJSON), &fieldValues); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid field values JSON: %v", err)), nil
	}

	// Convert template ID to UUID
	templateUUID, err := uuid.Parse(templateID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid template ID format: %v", err)), nil
	}

	// Get template
	template, err := workItemClient.GetTemplate(ctx, workitemtracking.GetTemplateArgs{
		Project:    &config.Project,
		Team:       nil,
		TemplateId: &templateUUID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get template: %v", err)), nil
	}

	// Create work item from template
	createArgs := workitemtracking.CreateWorkItemArgs{
		Type:    template.WorkItemTypeName,
		Project: &config.Project,
	}

	// Add template fields
	var operations []webapi.JsonPatchOperation
	for field, value := range *template.Fields {
		operations = append(operations, webapi.JsonPatchOperation{
			Op:    &webapi.OperationValues.Add,
			Path:  stringPtr("/fields/" + field),
			Value: value,
		})
	}

	// Override with provided field values
	for field, value := range fieldValues {
		operations = append(operations, webapi.JsonPatchOperation{
			Op:    &webapi.OperationValues.Add,
			Path:  stringPtr("/fields/" + field),
			Value: value,
		})
	}

	createArgs.Document = &operations

	workItem, err := workItemClient.CreateWorkItem(ctx, createArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create work item from template: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Created work item #%d from template", *workItem.Id)), nil
}

// Handler for adding attachment to work item
func handleAddWorkItemAttachment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))
	fileName := request.Params.Arguments["file_name"].(string)
	content := request.Params.Arguments["content"].(string)

	// Decode base64 content
	fileContent, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid base64 content: %v", err)), nil
	}

	// Create upload stream
	stream := bytes.NewReader(fileContent)

	// Upload attachment
	attachment, err := workItemClient.CreateAttachment(ctx, workitemtracking.CreateAttachmentArgs{
		UploadStream: stream,
		FileName:     &fileName,
		Project:      &config.Project,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to upload attachment: %v", err)), nil
	}

	// Add attachment reference to work item
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:   &webapi.OperationValues.Add,
				Path: stringPtr("/relations/-"),
				Value: map[string]interface{}{
					"rel": "AttachedFile",
					"url": *attachment.Url,
					"attributes": map[string]interface{}{
						"name": fileName,
					},
				},
			},
		},
	}

	_, err = workItemClient.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add attachment to work item: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Added attachment '%s' to work item #%d", fileName, id)), nil
}

// Handler for getting work item attachments
func handleGetWorkItemAttachments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))

	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
	}

	if workItem.Relations == nil {
		return mcp.NewToolResultText(fmt.Sprintf("No attachments found for work item #%d", id)), nil
	}

	var results []string
	for _, relation := range *workItem.Relations {
		if *relation.Rel == "AttachedFile" {
			name := (*relation.Attributes)["name"].(string)
			results = append(results, fmt.Sprintf("ID: %s\nName: %s\nURL: %s\n---",
				*relation.Url,
				name,
				*relation.Url))
		}
	}

	if len(results) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No attachments found for work item #%d", id)), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for removing attachment from work item
func handleRemoveWorkItemAttachment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))
	attachmentID := request.Params.Arguments["attachment_id"].(string)

	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
	}

	if workItem.Relations == nil {
		return mcp.NewToolResultError("Work item has no attachments"), nil
	}

	// Find the attachment relation index
	var relationIndex int = -1
	for i, relation := range *workItem.Relations {
		if *relation.Rel == "AttachedFile" && strings.Contains(*relation.Url, attachmentID) {
			relationIndex = i
			break
		}
	}

	if relationIndex == -1 {
		return mcp.NewToolResultError("Attachment not found"), nil
	}

	// Remove the attachment relation
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:   &webapi.OperationValues.Remove,
				Path: stringPtr(fmt.Sprintf("/relations/%d", relationIndex)),
			},
		},
	}

	_, err = workItemClient.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove attachment: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Removed attachment from work item #%d", id)), nil
}

func handleGetCurrentSprint(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	team, _ := request.Params.Arguments["team"].(string)
	if team == "" {
		team = config.Project + " Team" // Default team name
	}

	// Build the URL for the current iteration
	baseURL := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations",
		config.OrganizationURL,
		config.Project)

	queryParams := url.Values{}
	queryParams.Add("$timeframe", "current")
	queryParams.Add("api-version", "7.2-preview")

	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.SetBasicAuth("", config.PersonalAccessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get current sprint: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get current sprint. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var sprintResponse struct {
		Value []struct {
			Name      string    `json:"name"`
			StartDate time.Time `json:"startDate"`
			EndDate   time.Time `json:"finishDate"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&sprintResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	if len(sprintResponse.Value) == 0 {
		return mcp.NewToolResultText("No active sprint found"), nil
	}

	sprint := sprintResponse.Value[0]
	result := fmt.Sprintf("Current Sprint: %s\nStart Date: %s\nEnd Date: %s",
		sprint.Name,
		sprint.StartDate.Format("2006-01-02"),
		sprint.EndDate.Format("2006-01-02"))

	return mcp.NewToolResultText(result), nil
}

func handleGetSprints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	team, _ := request.Params.Arguments["team"].(string)
	includeCompleted, _ := request.Params.Arguments["include_completed"].(bool)
	if team == "" {
		team = config.Project + " Team"
	}

	// Build the URL for iterations
	baseURL := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations",
		config.OrganizationURL,
		config.Project)

	queryParams := url.Values{}
	if !includeCompleted {
		queryParams.Add("$timeframe", "current,future")
	}
	queryParams.Add("api-version", "7.2-preview")

	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.SetBasicAuth("", config.PersonalAccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get sprints: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get sprints. Status: %d", resp.StatusCode)), nil
	}

	var sprintResponse struct {
		Value []struct {
			Name      string    `json:"name"`
			StartDate time.Time `json:"startDate"`
			EndDate   time.Time `json:"finishDate"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&sprintResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	var results []string
	for _, sprint := range sprintResponse.Value {
		results = append(results, fmt.Sprintf("Sprint: %s\nStart: %s\nEnd: %s\n---",
			sprint.Name,
			sprint.StartDate.Format("2006-01-02"),
			sprint.EndDate.Format("2006-01-02")))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No sprints found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

type logWriter struct{}

func (w *logWriter) Write(bytes []byte) (int, error) {
	// Skip logging "Prompts not supported" errors
	if strings.Contains(string(bytes), "Prompts not supported") {
		return len(bytes), nil
	}
	return fmt.Print(string(bytes))
}
