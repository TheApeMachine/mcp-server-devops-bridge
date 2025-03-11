package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// WorkItemTool manages work items.
type WorkItemTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// NewWorkItemTool creates a new tool instance.
func NewWorkItemTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	wc, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		return nil
	}

	tool := &WorkItemTool{
		config: config,
		client: wc,
	}

	// Create tool handle with argument definitions.
	tool.handle = mcp.NewTool(
		"work_item",
		mcp.WithDescription("Manage work items"),
		mcp.WithString(
			"operation",
			mcp.Required(),
			mcp.Description("The operation to perform (create, update, query, get_details, manage_relations, get_related_work_items, add_comment, get_comments, get_fields, batch_create, batch_update, manage_tags, get_tags, get_templates, create_from_template, add_attachment, get_attachments, remove_attachment)"),
		),
		mcp.WithString("id", mcp.Description("The ID of the work item to manage")),
		mcp.WithString("field", mcp.Description("The field to update")),
		mcp.WithString("value", mcp.Description("The value to set for the field")),
		mcp.WithString("ids", mcp.Description("The IDs of the work items to manage")),
		mcp.WithString("source_id", mcp.Description("The ID of the source work item")),
		mcp.WithString("target_id", mcp.Description("The ID of the target work item")),
		mcp.WithString("relation_type", mcp.Description("The type of relation to manage")),
		mcp.WithString("file_name", mcp.Description("The name of the file to upload")),
		mcp.WithString("content", mcp.Description("The content of the file to upload")),
		mcp.WithString("comment", mcp.Description("The comment to add to the work item")),
		mcp.WithString("text", mcp.Description("The text to add to the work item")),
	)
	return tool
}

func (tool *WorkItemTool) Handle() mcp.Tool {
	return tool.handle
}

// OperationHandler defines a function type for handling an operation.
type OperationHandler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Map operations to their handler functions.
func (tool *WorkItemTool) operationHandlers() map[string]OperationHandler {
	return map[string]OperationHandler{
		"create":                 tool.handleCreateWorkItem,
		"update":                 tool.handleUpdateWorkItem,
		"query":                  tool.handleQueryWorkItems,
		"get_details":            tool.handleGetWorkItemDetails,
		"manage_relations":       tool.handleManageWorkItemRelations,
		"get_related_work_items": tool.handleGetRelatedWorkItems,
		"add_comment":            tool.handleAddWorkItemComment,
		"get_comments":           tool.handleGetWorkItemComments,
		"get_fields":             tool.handleGetWorkItemFields,
		"batch_create":           tool.handleBatchCreateWorkItems,
		"batch_update":           tool.handleBatchUpdateWorkItems,
		"manage_tags":            tool.handleManageWorkItemTags,
		"get_tags":               tool.handleGetWorkItemTags,
		"get_templates":          tool.handleGetWorkItemTemplates,
		"create_from_template":   tool.handleCreateFromTemplate,
		"add_attachment":         tool.handleAddWorkItemAttachment,
		"get_attachments":        tool.handleGetWorkItemAttachments,
		"remove_attachment":      tool.handleRemoveWorkItemAttachment,
	}
}

// Handler dispatches operations based on the "operation" argument.
func (tool *WorkItemTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	op, ok := request.Params.Arguments["operation"].(string)
	if !ok {
		return mcp.NewToolResultError("Missing operation parameter"), nil
	}

	handlers := tool.operationHandlers()
	if handler, exists := handlers[op]; exists {
		return handler(ctx, request)
	}

	return mcp.NewToolResultError("Unsupported operation: " + op), nil
}

// Helper to extract a string argument.
func getStringArg(req mcp.CallToolRequest, key string) (string, error) {
	val, ok := req.Params.Arguments[key]
	if !ok {
		return "", fmt.Errorf("missing argument: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("argument %s is not a string", key)
	}
	return str, nil
}

// Helper to extract a float64 argument.
func getFloat64Arg(req mcp.CallToolRequest, key string) (float64, error) {
	val, ok := req.Params.Arguments[key]
	if !ok {
		return 0, fmt.Errorf("missing argument: %s", key)
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("argument %s is not a number", key)
	}
	return f, nil
}

// Helper to extract an int argument from a float64.
func getIntArg(req mcp.CallToolRequest, key string) (int, error) {
	f, err := getFloat64Arg(req, key)
	if err != nil {
		return 0, err
	}
	return int(f), nil
}

// Helper for common error formatting
func handleError(err error, message string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", message, err))
}

// Helper to parse a comma-separated list of IDs
func parseIDs(idsStr string) ([]int, error) {
	idStrs := strings.Split(idsStr, ",")
	var ids []int

	for _, idStr := range idStrs {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			return nil, fmt.Errorf("invalid ID format: %s", idStr)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// A refactored update handler using helper functions.
func (tool *WorkItemTool) handleUpdateWorkItem(
	ctx context.Context,
	request mcp.CallToolRequest,
) (result *mcp.CallToolResult, err error) {
	id, err := getIntArg(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	field, err := getStringArg(request, "field")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	value, err := getStringArg(request, "value")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &tool.config.Project,
		Document: &[]webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Replace,
				Path:  stringPtr("/fields/" + field),
				Value: value,
			},
		},
	}

	workItem, err := tool.client.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return handleError(err, "Failed to update work item"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Updated work item #%d", *workItem.Id)), nil
}

func (tool *WorkItemTool) handleCreateWorkItem(
	ctx context.Context,
	request mcp.CallToolRequest,
) (result *mcp.CallToolResult, err error) {
	args := make(map[string]string)

	for _, arg := range []string{"type", "title", "description", "priority"} {
		value, err := getStringArg(request, arg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args[arg] = value
	}

	// Create document with required fields
	document := []webapi.JsonPatchOperation{
		addOperation("System.Title", args["title"]),
		addOperation("System.Description", args["description"]),
	}

	// Add optional priority if present
	if args["priority"] != "" {
		document = append(document, addOperation("Microsoft.VSTS.Common.Priority", args["priority"]))
	}

	createArgs := workitemtracking.CreateWorkItemArgs{
		Type:     stringPtr(args["type"]),
		Project:  &tool.config.Project,
		Document: &document,
	}

	workItem, err := tool.client.CreateWorkItem(ctx, createArgs)
	if err != nil {
		return handleError(err, "Failed to create work item"), nil
	}

	fields := *workItem.Fields
	var extractedTitle string
	if t, ok := fields["System.Title"].(string); ok {
		extractedTitle = t
	}

	return mcp.NewToolResultText(fmt.Sprintf("Created work item #%d: %s", *workItem.Id, extractedTitle)), nil
}

func (tool *WorkItemTool) handleQueryWorkItems(
	ctx context.Context,
	request mcp.CallToolRequest,
) (result *mcp.CallToolResult, err error) {
	query, err := getStringArg(request, "query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return handleError(err, "Failed to query work items"), nil
	}

	// Format results
	var results []string
	for _, item := range *queryResult.WorkItems {
		results = append(results, fmt.Sprintf("ID: %d", *item.Id))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleGetWorkItemDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsStr, err := getStringArg(request, "ids")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ids, err := parseIDs(idsStr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.All,
	})

	if err != nil {
		return handleError(err, "Failed to get work items"), nil
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

// Relation type mapping
var relationTypeMap = map[string]string{
	"parent":   "System.LinkTypes.Hierarchy-Reverse",
	"child":    "System.LinkTypes.Hierarchy-Forward",
	"children": "System.LinkTypes.Hierarchy-Forward", // Alias
	"related":  "System.LinkTypes.Related",
}

// Helper function to extract work item ID from relation URL
func extractWorkItemIDFromURL(url string) (int, error) {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid relation URL format")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

// Helper function to get a work item with relations
func (tool *WorkItemTool) getWorkItemWithRelations(ctx context.Context, id int) (*workitemtracking.WorkItem, error) {
	return tool.client.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})
}

// Helper function to resolve relation type from user-friendly name to Azure type
func resolveRelationType(relationType string) (string, bool) {
	azureRelationType, found := relationTypeMap[relationType]
	return azureRelationType, found
}

// Helper function to collect related work item IDs based on relation type
func collectRelatedWorkItemIDs(workItem *workitemtracking.WorkItem, relationType string) ([]int, []string, error) {
	if workItem.Relations == nil {
		return nil, nil, nil
	}

	var relatedIds []int
	var debugInfo []string

	azureRelationType, found := resolveRelationType(relationType)
	debugInfo = append(debugInfo, fmt.Sprintf("Looking for relation type: %s (mapped to: %s)",
		relationType, azureRelationType))

	for _, relation := range *workItem.Relations {
		debugInfo = append(debugInfo, fmt.Sprintf("Found relation of type: %s", *relation.Rel))

		if relationType == "all" || (found && *relation.Rel == azureRelationType) {
			relatedID, err := extractWorkItemIDFromURL(*relation.Url)
			if err == nil {
				relatedIds = append(relatedIds, relatedID)
			}
		}
	}

	return relatedIds, debugInfo, nil
}

// Helper function to get details of multiple work items
func (tool *WorkItemTool) getWorkItemDetails(ctx context.Context, ids []int) (*[]workitemtracking.WorkItem, error) {
	if len(ids) == 0 {
		return &[]workitemtracking.WorkItem{}, nil
	}

	return tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
	})
}

func (tool *WorkItemTool) handleManageWorkItemRelations(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sourceID, err := getIntArg(request, "source_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	targetID, err := getIntArg(request, "target_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	relationType, err := getStringArg(request, "relation_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	operation, err := getStringArg(request, "operation")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	azureRelationType, found := resolveRelationType(relationType)
	if !found {
		return mcp.NewToolResultError(fmt.Sprintf("Unknown relation type: %s", relationType)), nil
	}

	var ops []webapi.JsonPatchOperation
	if operation == "add" {
		ops = []webapi.JsonPatchOperation{
			addOperation("relations/-", map[string]any{
				"rel": azureRelationType,
				"url": fmt.Sprintf("%s/_apis/wit/workItems/%d", tool.config.OrganizationURL, targetID),
				"attributes": map[string]any{
					"comment": "Added via MCP",
				},
			}),
		}
	} else {
		// For remove, we need to first get the work item to find the relation index
		workItem, err := tool.getWorkItemWithRelations(ctx, sourceID)
		if err != nil {
			return handleError(err, "Failed to get work item"), nil
		}

		if workItem.Relations == nil {
			return mcp.NewToolResultError("Work item has no relations"), nil
		}

		for i, relation := range *workItem.Relations {
			if *relation.Rel == azureRelationType {
				targetUrl := fmt.Sprintf("%s/_apis/wit/workItems/%d", tool.config.OrganizationURL, targetID)
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
		Project:  &tool.config.Project,
		Document: &ops,
	}

	_, err = tool.client.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return handleError(err, "Failed to update work item relations"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully %sd %s relationship", operation, relationType)), nil
}

func (tool *WorkItemTool) handleGetRelatedWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := getIntArg(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	relationType, err := getStringArg(request, "relation_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	workItem, err := tool.getWorkItemWithRelations(ctx, id)
	if err != nil {
		return handleError(err, "Failed to get work item"), nil
	}

	relatedIds, debugInfo, _ := collectRelatedWorkItemIDs(workItem, relationType)

	if len(relatedIds) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Debug info:\n%s\n\nNo matching related items found",
			strings.Join(debugInfo, "\n"))), nil
	}

	// Get details of related items
	relatedItems, err := tool.getWorkItemDetails(ctx, relatedIds)
	if err != nil {
		return handleError(err, "Failed to get related items"), nil
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

func (tool *WorkItemTool) handleAddWorkItemComment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := getIntArg(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	text, err := getStringArg(request, "text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Add comment as a discussion by updating the Discussion field
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &tool.config.Project,
		Document: &[]webapi.JsonPatchOperation{
			addOperation("System.History", text),
		},
	}

	workItem, err := tool.client.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return handleError(err, "Failed to add comment"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Added comment to work item #%d", *workItem.Id)), nil
}

func (tool *WorkItemTool) handleGetWorkItemComments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := getIntArg(request, "id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	comments, err := tool.client.GetComments(ctx, workitemtracking.GetCommentsArgs{
		Project:    &tool.config.Project,
		WorkItemId: &id,
	})

	if err != nil {
		return handleError(err, "Failed to get comments"), nil
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

func (tool *WorkItemTool) handleGetWorkItemFields(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := getIntArg(request, "work_item_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get the work item's details
	workItem, err := tool.client.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: &tool.config.Project,
	})

	if err != nil {
		return handleError(err, "Failed to get work item details"), nil
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

func addOperation(field string, value any) webapi.JsonPatchOperation {
	return webapi.JsonPatchOperation{
		Op:    &webapi.OperationValues.Add,
		Path:  stringPtr("/fields/" + field),
		Value: value,
	}
}

func (tool *WorkItemTool) handleBatchCreateWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	itemsJSON, err := getStringArg(request, "items")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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
		// Create document with required fields
		document := []webapi.JsonPatchOperation{
			addOperation("System.Title", item.Title),
			addOperation("System.Description", item.Description),
		}

		// Add optional priority if present
		if item.Priority != "" {
			document = append(document, addOperation("Microsoft.VSTS.Common.Priority", item.Priority))
		}

		createArgs := workitemtracking.CreateWorkItemArgs{
			Type:     &item.Type,
			Project:  &tool.config.Project,
			Document: &document,
		}

		workItem, err := tool.client.CreateWorkItem(ctx, createArgs)
		if err != nil {
			results = append(results, fmt.Sprintf("Failed to create '%s': %v", item.Title, err))
			continue
		}
		results = append(results, fmt.Sprintf("Created work item #%d: %s", *workItem.Id, item.Title))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Field name mapping
var fieldMap = map[string]string{
	"Title":       "System.Title",
	"Description": "System.Description",
	"State":       "System.State",
	"Priority":    "Microsoft.VSTS.Common.Priority",
}

func (tool *WorkItemTool) handleBatchUpdateWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	updatesJSON, err := getStringArg(request, "updates")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var updates []struct {
		ID    int    `json:"id"`
		Field string `json:"field"`
		Value string `json:"value"`
	}

	if err := json.Unmarshal([]byte(updatesJSON), &updates); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid JSON format: %v", err)), nil
	}

	var results []string
	for _, update := range updates {
		systemField, ok := fieldMap[update.Field]
		if !ok {
			// Try direct field name if not in map
			systemField = update.Field
		}

		updateArgs := workitemtracking.UpdateWorkItemArgs{
			Id:      &update.ID,
			Project: &tool.config.Project,
			Document: &[]webapi.JsonPatchOperation{
				{
					Op:    &webapi.OperationValues.Replace,
					Path:  stringPtr("/fields/" + systemField),
					Value: update.Value,
				},
			},
		}

		workItem, err := tool.client.UpdateWorkItem(ctx, updateArgs)
		if err != nil {
			results = append(results, fmt.Sprintf("Failed to update #%d: %v", update.ID, err))
			continue
		}
		results = append(results, fmt.Sprintf("Updated work item #%d", *workItem.Id))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}
