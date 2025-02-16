package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

func addWorkItemTools(s *server.MCPServer) {
	// Add WIQL Query Format Prompt
	s.AddPrompt(mcp.NewPrompt("wiql_query_format",
		mcp.WithPromptDescription("Helper for formatting WIQL queries for common scenarios"),
		mcp.WithArgument("query_type",
			mcp.ArgumentDescription("Type of query to format (current_sprint, assigned_to_me, etc)"),
			mcp.RequiredArgument(),
		),
		mcp.WithArgument("additional_fields",
			mcp.ArgumentDescription("Additional fields to include in the SELECT clause"),
		),
	), handleWiqlQueryFormatPrompt)

	// Create Work Item
	createWorkItemTool := mcp.NewTool("create_work_item",
		mcp.WithDescription("Create a new work item in Azure DevOps"),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Type of work item (Epic, Feature, User Story, Task, Bug)"),
			mcp.Enum("Epic", "Feature", "User Story", "Task", "Bug"),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Title of the work item"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Description of the work item"),
		),
		mcp.WithString("priority",
			mcp.Description("Priority of the work item (1-4)"),
			mcp.Enum("1", "2", "3", "4"),
		),
	)

	s.AddTool(createWorkItemTool, handleCreateWorkItem)

	// Update Work Item
	updateWorkItemTool := mcp.NewTool("update_work_item",
		mcp.WithDescription("Update an existing work item in Azure DevOps"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item to update"),
		),
		mcp.WithString("field",
			mcp.Required(),
			mcp.Description("Field to update (Title, Description, State, Priority)"),
			mcp.Enum("Title", "Description", "State", "Priority"),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("New value for the field"),
		),
	)

	s.AddTool(updateWorkItemTool, handleUpdateWorkItem)

	// Query Work Items
	queryWorkItemsTool := mcp.NewTool("query_work_items",
		mcp.WithDescription("Query work items using WIQL"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("WIQL query string"),
		),
	)

	s.AddTool(queryWorkItemsTool, handleQueryWorkItems)

	// Get Work Item Details
	getWorkItemTool := mcp.NewTool("get_work_item_details",
		mcp.WithDescription("Get detailed information about work items"),
		mcp.WithString("ids",
			mcp.Required(),
			mcp.Description("Comma-separated list of work item IDs"),
		),
	)
	s.AddTool(getWorkItemTool, handleGetWorkItemDetails)

	// Manage Work Item Relations
	manageRelationsTool := mcp.NewTool("manage_work_item_relations",
		mcp.WithDescription("Manage relationships between work items"),
		mcp.WithNumber("source_id",
			mcp.Required(),
			mcp.Description("ID of the source work item"),
		),
		mcp.WithNumber("target_id",
			mcp.Required(),
			mcp.Description("ID of the target work item"),
		),
		mcp.WithString("relation_type",
			mcp.Required(),
			mcp.Description("Type of relationship to manage"),
			mcp.Enum("parent", "child", "related"),
		),
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("Operation to perform"),
			mcp.Enum("add", "remove"),
		),
	)
	s.AddTool(manageRelationsTool, handleManageWorkItemRelations)

	// Get Related Work Items
	getRelatedItemsTool := mcp.NewTool("get_related_work_items",
		mcp.WithDescription("Get related work items"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item to get relations for"),
		),
		mcp.WithString("relation_type",
			mcp.Required(),
			mcp.Description("Type of relationships to get"),
			mcp.Enum("parent", "children", "related", "all"),
		),
	)
	s.AddTool(getRelatedItemsTool, handleGetRelatedWorkItems)

	// Comment Management Tool (as Discussion)
	addCommentTool := mcp.NewTool("add_work_item_comment",
		mcp.WithDescription("Add a comment to a work item as a discussion"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Comment text"),
		),
	)
	s.AddTool(addCommentTool, handleAddWorkItemComment)

	getCommentsTool := mcp.NewTool("get_work_item_comments",
		mcp.WithDescription("Get comments for a work item"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
	)
	s.AddTool(getCommentsTool, handleGetWorkItemComments)

	// Field Management Tool
	getFieldsTool := mcp.NewTool("get_work_item_fields",
		mcp.WithDescription("Get available work item fields and their current values"),
		mcp.WithNumber("work_item_id",
			mcp.Required(),
			mcp.Description("ID of the work item to examine fields from"),
		),
		mcp.WithString("field_name",
			mcp.Description("Optional field name to filter (case-insensitive partial match)"),
		),
	)
	s.AddTool(getFieldsTool, handleGetWorkItemFields)

	// Batch Operations Tools
	batchCreateTool := mcp.NewTool("batch_create_work_items",
		mcp.WithDescription("Create multiple work items in a single operation"),
		mcp.WithString("items",
			mcp.Required(),
			mcp.Description("JSON array of work items to create, each containing type, title, and description"),
		),
	)
	s.AddTool(batchCreateTool, handleBatchCreateWorkItems)

	batchUpdateTool := mcp.NewTool("batch_update_work_items",
		mcp.WithDescription("Update multiple work items in a single operation"),
		mcp.WithString("updates",
			mcp.Required(),
			mcp.Description("JSON array of updates, each containing id, field, and value"),
		),
	)
	s.AddTool(batchUpdateTool, handleBatchUpdateWorkItems)

	// Tag Management Tools
	manageTags := mcp.NewTool("manage_work_item_tags",
		mcp.WithDescription("Add or remove tags from a work item"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("Operation to perform"),
			mcp.Enum("add", "remove"),
		),
		mcp.WithString("tags",
			mcp.Required(),
			mcp.Description("Comma-separated list of tags"),
		),
	)
	s.AddTool(manageTags, handleManageWorkItemTags)

	getTagsTool := mcp.NewTool("get_work_item_tags",
		mcp.WithDescription("Get tags for a work item"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
	)
	s.AddTool(getTagsTool, handleGetWorkItemTags)

	// Work Item Template Tools
	getTemplatesTool := mcp.NewTool("get_work_item_templates",
		mcp.WithDescription("Get available work item templates"),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Type of work item to get templates for"),
			mcp.Enum("Epic", "Feature", "User Story", "Task", "Bug"),
		),
	)
	s.AddTool(getTemplatesTool, handleGetWorkItemTemplates)

	createFromTemplateTool := mcp.NewTool("create_from_template",
		mcp.WithDescription("Create a work item from a template"),
		mcp.WithString("template_id",
			mcp.Required(),
			mcp.Description("ID of the template to use"),
		),
		mcp.WithString("field_values",
			mcp.Required(),
			mcp.Description("JSON object of field values to override template defaults"),
		),
	)
	s.AddTool(createFromTemplateTool, handleCreateFromTemplate)

	// Attachment Management Tools
	addAttachmentTool := mcp.NewTool("add_work_item_attachment",
		mcp.WithDescription("Add an attachment to a work item"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
		mcp.WithString("file_name",
			mcp.Required(),
			mcp.Description("Name of the file to attach"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Base64 encoded content of the file"),
		),
	)
	s.AddTool(addAttachmentTool, handleAddWorkItemAttachment)

	getAttachmentsTool := mcp.NewTool("get_work_item_attachments",
		mcp.WithDescription("Get attachments for a work item"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
	)
	s.AddTool(getAttachmentsTool, handleGetWorkItemAttachments)

	removeAttachmentTool := mcp.NewTool("remove_work_item_attachment",
		mcp.WithDescription("Remove an attachment from a work item"),
		mcp.WithNumber("id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
		mcp.WithString("attachment_id",
			mcp.Required(),
			mcp.Description("ID of the attachment to remove"),
		),
	)
	s.AddTool(removeAttachmentTool, handleRemoveWorkItemAttachment)

	// Sprint Management Tools
	getCurrentSprintTool := mcp.NewTool("get_current_sprint",
		mcp.WithDescription("Get details about the current sprint"),
		mcp.WithString("team",
			mcp.Description("Team name (optional, defaults to project's default team)"),
		),
	)
	s.AddTool(getCurrentSprintTool, handleGetCurrentSprint)

	getSprintsTool := mcp.NewTool("get_sprints",
		mcp.WithDescription("Get list of sprints"),
		mcp.WithString("team",
			mcp.Description("Team name (optional, defaults to project's default team)"),
		),
		mcp.WithBoolean("include_completed",
			mcp.Description("Whether to include completed sprints"),
		),
	)
	s.AddTool(getSprintsTool, handleGetSprints)

	// Add a new prompt for work item descriptions
	s.AddPrompt(mcp.NewPrompt("format_work_item_description",
		mcp.WithPromptDescription("Format a work item description using proper HTML for Azure DevOps"),
		mcp.WithArgument("description",
			mcp.ArgumentDescription("The description text to format"),
			mcp.RequiredArgument(),
		),
	), handleFormatWorkItemDescription)
}

func handleUpdateWorkItem(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int(request.Params.Arguments["id"].(float64))
	field := request.Params.Arguments["field"].(string)
	value := request.Params.Arguments["value"].(string)

	// Instead of using a fixed map, directly use the field name
	// This allows any valid Azure DevOps field to be used
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Replace,
				Path:  stringPtr("/fields/" + field),
				Value: value,
			},
		},
	}

	workItem, err := workItemClient.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update work item: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Updated work item #%d", *workItem.Id)), nil
}

func handleCreateWorkItem(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workItemType := request.Params.Arguments["type"].(string)
	title := request.Params.Arguments["title"].(string)
	description := request.Params.Arguments["description"].(string)
	priority, hasPriority := request.Params.Arguments["priority"].(string)

	// Create the work item
	createArgs := workitemtracking.CreateWorkItemArgs{
		Type:    &workItemType,
		Project: &config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Add,
				Path:  stringPtr("/fields/System.Title"),
				Value: title,
			},
			{
				Op:    &webapi.OperationValues.Add,
				Path:  stringPtr("/fields/System.Description"),
				Value: description,
			},
		},
	}

	if hasPriority {
		doc := append(*createArgs.Document, webapi.JsonPatchOperation{
			Op:    &webapi.OperationValues.Add,
			Path:  stringPtr("/fields/Microsoft.VSTS.Common.Priority"),
			Value: priority,
		})
		createArgs.Document = &doc
	}

	workItem, err := workItemClient.CreateWorkItem(ctx, createArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create work item: %v", err)), nil
	}

	fields := *workItem.Fields
	var extractedTitle string
	if t, ok := fields["System.Title"].(string); ok {
		extractedTitle = t
	}
	return mcp.NewToolResultText(fmt.Sprintf("Created work item #%d: %s", *workItem.Id, extractedTitle)), nil
}

func handleQueryWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.Params.Arguments["query"].(string)

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := workItemClient.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to query work items: %v", err)), nil
	}

	// Format results
	var results []string
	for _, item := range *queryResult.WorkItems {
		results = append(results, fmt.Sprintf("ID: %d", *item.Id))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func handleWiqlQueryFormatPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	queryType, exists := request.Params.Arguments["query_type"]
	if !exists {
		return nil, fmt.Errorf("query_type is required")
	}

	additionalFields := request.Params.Arguments["additional_fields"]

	baseFields := "[System.Id], [System.Title], [System.WorkItemType], [System.State], [System.AssignedTo]"
	if additionalFields != "" {
		baseFields += ", " + additionalFields
	}

	var template string
	var explanation string

	switch queryType {
	case "current_sprint":
		template = fmt.Sprintf("SELECT %s FROM WorkItems WHERE [System.IterationPath] = @currentIteration('fanapp')", baseFields)
		explanation = "This query gets all work items in the current sprint. The @currentIteration macro automatically resolves to the current sprint path."

	case "assigned_to_me":
		template = fmt.Sprintf("SELECT %s FROM WorkItems WHERE [System.AssignedTo] = @me AND [System.State] <> 'Closed'", baseFields)
		explanation = "This query gets all active work items assigned to the current user. The @me macro automatically resolves to the current user."

	case "active_bugs":
		template = fmt.Sprintf("SELECT %s FROM WorkItems WHERE [System.WorkItemType] = 'Bug' AND [System.State] <> 'Closed' ORDER BY [Microsoft.VSTS.Common.Priority]", baseFields)
		explanation = "This query gets all active bugs, ordered by priority."

	case "blocked_items":
		template = fmt.Sprintf("SELECT %s FROM WorkItems WHERE [System.State] <> 'Closed' AND [Microsoft.VSTS.Common.Blocked] = 'Yes'", baseFields)
		explanation = "This query gets all work items that are marked as blocked."

	case "recent_activity":
		template = fmt.Sprintf("SELECT %s FROM WorkItems WHERE [System.ChangedDate] > @today-7 ORDER BY [System.ChangedDate] DESC", baseFields)
		explanation = "This query gets all work items modified in the last 7 days, ordered by most recent first."
	}

	return mcp.NewGetPromptResult(
		"WIQL Query Format Helper",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				"system",
				mcp.NewTextContent("You are a WIQL query expert. Help format queries for Azure DevOps work items."),
			),
			mcp.NewPromptMessage(
				"assistant",
				mcp.NewTextContent(fmt.Sprintf("Here's a template for a %s query:\n\n```sql\n%s\n```\n\n%s\n\nCommon WIQL Tips:\n- Use square brackets [] around field names\n- Common macros: @me, @today, @currentIteration\n- Date arithmetic: @today+/-n\n- String comparison is case-insensitive\n- Use 'Contains' for partial matches", queryType, template, explanation)),
			),
		},
	), nil
}

func handleFormatWorkItemDescription(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	description := request.Params.Arguments["description"]
	return mcp.NewGetPromptResult(
		"Azure DevOps Work Item Description Formatter",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				"system",
				mcp.NewTextContent("You format work item descriptions for Azure DevOps. Use proper HTML formatting with <ul>, <li> for bullet points, <p> for paragraphs, and <br> for line breaks."),
			),
			mcp.NewPromptMessage(
				"assistant",
				mcp.NewTextContent(fmt.Sprintf("Here's your description formatted with HTML:\n\n<ul>\n%s\n</ul>",
					strings.Join(strings.Split(description, "-"), "</li>\n<li>"))),
			),
		},
	), nil
}
