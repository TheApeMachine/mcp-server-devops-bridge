package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// Handler for getting work item templates
func (tool *WorkItemTool) handleGetWorkItemTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely get the workItemType with a fallback to empty string if not provided
	workItemTypeValue, exists := request.Params.Arguments["type"]
	if !exists || workItemTypeValue == nil {
		return mcp.NewToolResultError("Missing required parameter: 'type'. Please specify the work item type to get templates for."), nil
	}
	
	workItemType, ok := workItemTypeValue.(string)
	if !ok {
		return mcp.NewToolResultError("Parameter 'type' must be a string."), nil
	}

	templates, err := tool.client.GetTemplates(ctx, workitemtracking.GetTemplatesArgs{
		Project:          &tool.config.Project,
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
func (tool *WorkItemTool) handleCreateFromTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely get the templateID parameter
	templateIDValue, exists := request.Params.Arguments["template_id"]
	if !exists || templateIDValue == nil {
		return mcp.NewToolResultError("Missing required parameter: 'template_id'. Please specify the template ID."), nil
	}
	
	templateID, ok := templateIDValue.(string)
	if !ok {
		return mcp.NewToolResultError("Parameter 'template_id' must be a string."), nil
	}
	
	// Safely get the fieldValuesJSON parameter
	fieldValuesJSONValue, exists := request.Params.Arguments["field_values"]
	if !exists || fieldValuesJSONValue == nil {
		return mcp.NewToolResultError("Missing required parameter: 'field_values'. Please provide field values in JSON format."), nil
	}
	
	fieldValuesJSON, ok := fieldValuesJSONValue.(string)
	if !ok {
		return mcp.NewToolResultError("Parameter 'field_values' must be a string containing JSON."), nil
	}

	var fieldValues map[string]any
	if err := json.Unmarshal([]byte(fieldValuesJSON), &fieldValues); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid field values JSON: %v", err)), nil
	}

	// Convert template ID to UUID
	templateUUID, err := uuid.Parse(templateID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid template ID format: %v", err)), nil
	}

	// Get template
	template, err := tool.client.GetTemplate(ctx, workitemtracking.GetTemplateArgs{
		Project:    &tool.config.Project,
		Team:       nil,
		TemplateId: &templateUUID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get template: %v", err)), nil
	}

	// Create work item from template
	createArgs := workitemtracking.CreateWorkItemArgs{
		Type:    template.WorkItemTypeName,
		Project: &tool.config.Project,
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

	workItem, err := tool.client.CreateWorkItem(ctx, createArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create work item from template: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Created work item #%d from template", *workItem.Id)), nil
}
