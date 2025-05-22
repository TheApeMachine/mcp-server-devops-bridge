package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

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

	// Create tool handle with argument definitions and improved documentation.
	tool.handle = mcp.NewTool(
		"work_item",
		mcp.WithDescription("Manage Azure DevOps work items (tasks, bugs, stories, epics, etc.)"),
		mcp.WithString(
			"operation",
			mcp.Required(),
			mcp.Description("The operation to perform. For help, use 'get_help' or 'get_examples' operations."),
		),
		// Documentation helpers
		mcp.WithString("filter", mcp.Description("Optional filter text to limit results (used with list_fields)")),
		mcp.WithString("states", mcp.Description("Comma-separated list of states to filter by (e.g., 'DOING,REVIEW') - used with find_work_items")),
		mcp.WithString("has_parent", mcp.Description("Set to 'true' or 'false' to filter by parent relationship - used with find_work_items")),
		mcp.WithString("parent_type", mcp.Description("The type of parent to check for (e.g., 'Epic') - used with find_work_items")),
		mcp.WithString("query", mcp.Description("WIQL query string for searching work items - used with query operation")),
		mcp.WithString("search_text", mcp.Description("Text to search for in work item titles and descriptions - used with search operation")),
		mcp.WithString("format", mcp.Description("Response format: 'text' (default) or 'json'")),
		mcp.WithString("page_size", mcp.Description("Number of items per page (default: 20)")),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),

		// Common parameters for most operations
		mcp.WithString("id", mcp.Description("The ID of the work item to manage")),
		mcp.WithString("ids", mcp.Description("Comma-separated list of work item IDs (e.g., '123,456,789')")),
		mcp.WithString("field", mcp.Description("The field to update (e.g., 'System.Title', 'System.State')")),
		mcp.WithString("value", mcp.Description("The value to set for the field")),

		// Relation management
		mcp.WithString("source_id", mcp.Description("The ID of the source work item in a relationship")),
		mcp.WithString("target_id", mcp.Description("The ID of the target work item in a relationship")),
		mcp.WithString("relation_type", mcp.Description("Type of relation: parent, child, related")),

		// File/attachment operations
		mcp.WithString("file_name", mcp.Description("The name of the file to upload as attachment")),
		mcp.WithString("content", mcp.Description("Base64-encoded content of the file to upload")),

		// Comments
		mcp.WithString("text", mcp.Description("Text content to add as comment")),
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
		"get_help":               tool.handleGetHelp,
		"get_examples":           tool.handleGetExamples,
		"list_fields":            tool.handleListFields,
		"find_work_items":        tool.handleFindWorkItems,
		"get_states":             tool.handleGetStates,
		"get_work_item_types":    tool.handleGetWorkItemTypes,
		"search":                 tool.handleSearchWorkItems,
		"find_orphaned_items":    tool.handleFindOrphanedItems,
		"find_blocked_items":     tool.handleFindBlockedItems,
		"find_overdue_items":     tool.handleFindOverdueItems,
		"find_sprint_items":      tool.handleFindSprintItems,
	}
}

// Handler dispatches operations based on the "operation" argument.
func (tool *WorkItemTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		op string
		ok bool
	)

	if op, ok = request.Params.Arguments["operation"].(string); !ok {
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
	var (
		val any
		str string
		ok  bool
	)

	if val, ok = req.Params.Arguments[key]; !ok {
		return "", fmt.Errorf("missing argument: %s", key)
	}

	str, ok = val.(string)

	if !ok {
		return "", fmt.Errorf("argument %s is not a string", key)
	}

	return str, nil
}

// Helper to extract a float64 argument.
func getFloat64Arg(req mcp.CallToolRequest, key string) (float64, error) {
	var (
		val any
		f   float64
		ok  bool
	)

	if val, ok = req.Params.Arguments[key]; !ok {
		return 0, fmt.Errorf("missing argument: %s", key)
	}

	f, ok = val.(float64)

	if !ok {
		return 0, fmt.Errorf("argument %s is not a number", key)
	}

	return f, nil
}

// Helper to extract an int argument from a float64.
func getIntArg(req mcp.CallToolRequest, key string) (int, error) {
	var (
		f   float64
		err error
	)

	if f, err = getFloat64Arg(req, key); err != nil {
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
	var (
		idStrs []string
		ids    []int
	)

	idStrs = strings.Split(idsStr, ",")

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
		exampleQuery := "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'"
		return mcp.NewToolResultError(fmt.Sprintf(`
Missing "query" parameter. Please provide a valid WIQL query string.

Example query: %s

For more examples, use the "get_examples" operation or "get_help" with parameter "operation": "query"
`, exampleQuery)), nil
	}

	// Validate that query contains basic WIQL elements
	if !strings.Contains(strings.ToUpper(query), "SELECT") || !strings.Contains(strings.ToUpper(query), "FROM") {
		return mcp.NewToolResultError(fmt.Sprintf(`
Invalid WIQL query format. Query must contain SELECT and FROM clauses.

Your query: %s

Example of valid query: SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'

For more examples, use the "get_examples" operation.
`, query)), nil
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Try to provide a more helpful error message with suggestions
		errorMsg := err.Error()
		suggestion := ""

		if strings.Contains(errorMsg, "TF51005") {
			suggestion = "Check that field names are enclosed in square brackets, e.g. [System.State]"
		} else if strings.Contains(errorMsg, "TF51004") {
			suggestion = "Check for syntax errors in your query. Make sure field names and values are correctly formatted."
		}

		errorResponse := fmt.Sprintf("Failed to query work items: %v\n\nYour query: %s", err, query)
		if suggestion != "" {
			errorResponse += "\n\nSuggestion: " + suggestion
		}
		errorResponse += "\n\nFor example queries, use the 'get_examples' operation."

		return mcp.NewToolResultError(errorResponse), nil
	}

	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No work items found for query: %s", query)), nil
	}

	// Format results
	var results []string
	for _, item := range *queryResult.WorkItems {
		results = append(results, fmt.Sprintf("ID: %d", *item.Id))
	}

	// Add suggestion to get more details if there are results
	if len(results) > 0 {
		results = append(results, fmt.Sprintf("\nTo get details of these work items, use the 'get_details' operation with 'ids': \"%s\"",
			strings.Join(strings.Split(strings.Join(results, ","), "ID: "), ",")))
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleGetWorkItemDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsStr, err := getStringArg(request, "ids")
	if err != nil {
		return mcp.NewToolResultError(`
Missing "ids" parameter. Please provide a comma-separated list of work item IDs.

Example: "ids": "123,456,789"
`), nil
	}

	ids, err := parseIDs(idsStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(`
Invalid ID format in "%s". IDs must be comma-separated numbers.

Example of valid format: "123,456,789"
`, idsStr)), nil
	}

	if len(ids) == 0 {
		return mcp.NewToolResultError("No valid IDs provided. Please provide at least one work item ID."), nil
	}

	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.All,
	})

	if err != nil {
		return handleError(err, fmt.Sprintf("Failed to get work items for IDs: %s", idsStr)), nil
	}

	if len(*workItems) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No work items found for IDs: %s", idsStr)), nil
	}

	var results []string
	for _, item := range *workItems {
		fields := *item.Fields
		title, _ := fields["System.Title"].(string)
		description, _ := fields["System.Description"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Build a more comprehensive result with more fields
		var fieldDetails []string
		fieldDetails = append(fieldDetails, fmt.Sprintf("ID: %d", *item.Id))
		fieldDetails = append(fieldDetails, fmt.Sprintf("Title: %s", title))
		fieldDetails = append(fieldDetails, fmt.Sprintf("Type: %s", workItemType))
		fieldDetails = append(fieldDetails, fmt.Sprintf("State: %s", state))

		// Check for assignee
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				fieldDetails = append(fieldDetails, fmt.Sprintf("Assigned To: %s", displayName))
			}
		}

		// Add other useful fields if they exist
		if priority, ok := fields["Microsoft.VSTS.Common.Priority"].(string); ok {
			fieldDetails = append(fieldDetails, fmt.Sprintf("Priority: %s", priority))
		}

		// Add tags if they exist
		if tags, ok := fields["System.Tags"].(string); ok && tags != "" {
			fieldDetails = append(fieldDetails, fmt.Sprintf("Tags: %s", tags))
		}

		// Add description at the end since it can be long
		if description != "" {
			fieldDetails = append(fieldDetails, fmt.Sprintf("Description: %s", description))
		}

		// Check for parent/child relations
		if item.Relations != nil {
			var parentIDs, childIDs []string

			for _, relation := range *item.Relations {
				if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
					// This is a parent relationship
					relatedID, _ := extractWorkItemIDFromURL(*relation.Url)
					parentIDs = append(parentIDs, fmt.Sprintf("%d", relatedID))
				} else if *relation.Rel == "System.LinkTypes.Hierarchy-Forward" {
					// This is a child relationship
					relatedID, _ := extractWorkItemIDFromURL(*relation.Url)
					childIDs = append(childIDs, fmt.Sprintf("%d", relatedID))
				}
			}

			if len(parentIDs) > 0 {
				fieldDetails = append(fieldDetails, fmt.Sprintf("Parent IDs: %s", strings.Join(parentIDs, ", ")))
			}

			if len(childIDs) > 0 {
				fieldDetails = append(fieldDetails, fmt.Sprintf("Child IDs: %s", strings.Join(childIDs, ", ")))
			}
		}

		result := strings.Join(fieldDetails, "\n") + "\n---\n"
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

// Helper function to get documentation about all operations
func (tool *WorkItemTool) handleGetHelp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if specific operation was requested
	operation, hasOperation := request.Params.Arguments["operation"].(string)

	helpText := "## Azure DevOps Work Item Tool Help\n\n"

	if hasOperation {
		// Provide detailed help for a specific operation
		switch operation {
		case "query":
			return mcp.NewToolResultText(`
## Query Work Items Help

The query operation allows you to search for work items using WIQL (Work Item Query Language).

### Parameters:
- operation: "query" (required)
- query: A WIQL query string (required)

### WIQL Query Format:
WIQL queries follow this general format:
SELECT [Fields] FROM WorkItems WHERE [Conditions] [ORDER BY Field]

### Example WIQL Queries:
- Find items in DOING state:
  SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'

- Find items in DOING or REVIEW states:
  SELECT [System.Id] FROM WorkItems WHERE [System.State] IN ('DOING', 'REVIEW')

- Find items without parent links:
  SELECT [System.Id] FROM WorkItems 
  WHERE [System.State] IN ('DOING', 'REVIEW') 
  AND [System.Id] NOT IN (
    SELECT [System.Id] FROM WorkItemLinks 
    WHERE [Source].[System.WorkItemType] = 'Epic' 
    AND [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Forward' 
    MODE (MustContain)
  )
`), nil

		case "find_work_items":
			return mcp.NewToolResultText(`
## Find Work Items Help

The find_work_items operation simplifies finding work items in specific states without requiring WIQL knowledge.

### Parameters:
- operation: "find_work_items" (required)
- states: Comma-separated list of states to search for (required, e.g., "DOING,REVIEW")
- has_parent: Set to "true" or "false" to filter by parent relationship (optional)
- parent_type: The type of parent to check for (optional, default is "Epic")
- limit: Maximum number of results to return (optional, default is 100)

### Examples:
- Find items in DOING or REVIEW states:
  operation: "find_work_items", states: "DOING,REVIEW"

- Find items in DOING state without Epic parents:
  operation: "find_work_items", states: "DOING", has_parent: "false", parent_type: "Epic"
`), nil

		case "list_fields":
			return mcp.NewToolResultText(`
## List Fields Help

The list_fields operation retrieves available field names and their reference names.

### Parameters:
- operation: "list_fields" (required)
- filter: Optional text to filter field names (optional)

### Examples:
- Get all fields:
  operation: "list_fields"

- Get fields related to state:
  operation: "list_fields", filter: "state"
`), nil

		// Add help for other operations as needed
		default:
			return mcp.NewToolResultText(fmt.Sprintf("No detailed help available for operation '%s'", operation)), nil
		}
	}

	// General help for all operations
	helpText += "### Available Operations:\n\n"
	helpText += "- query: Search for work items using WIQL\n"
	helpText += "- get_details: Get details of specific work items by IDs\n"
	helpText += "- find_work_items: Helper to find work items by state and parent relationships\n"
	helpText += "- list_fields: List available fields without needing a work item ID\n"
	helpText += "- get_help: Get documentation about operations\n"
	helpText += "- get_examples: Get example WIQL queries and usage patterns\n"
	helpText += "- search: Search for work items by text in titles and descriptions\n"
	helpText += "- get_states: Get all available work item states\n"
	helpText += "- get_work_item_types: Get all available work item types\n"
	helpText += "- find_orphaned_items: Find work items without parent links\n"
	helpText += "- find_blocked_items: Find work items marked as blocked\n"
	helpText += "- find_overdue_items: Find work items with past due dates\n"
	helpText += "- find_sprint_items: Find work items in the current sprint\n"
	helpText += "- create: Create a new work item\n"
	helpText += "- update: Update a work item field\n"
	helpText += "- manage_relations: Add or remove relations between work items\n"
	helpText += "- get_related_work_items: Get work items related to a specific item\n"

	helpText += "\n\nFor detailed help on a specific operation, use: operation: \"get_help\", operation: \"OPERATION_NAME\""

	return mcp.NewToolResultText(helpText), nil
}

// Helper function to provide example queries and usage patterns
func (tool *WorkItemTool) handleGetExamples(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	examples := `
## Azure DevOps Work Item Tool Examples

### WIQL Query Examples

1. Find all work items in DOING state:
   {
     "operation": "query",
     "query": "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'"
   }

2. Find items in DOING or REVIEW states:
   {
     "operation": "query",
     "query": "SELECT [System.Id] FROM WorkItems WHERE [System.State] IN ('DOING', 'REVIEW')"
   }

3. Find user stories in DOING state:
   {
     "operation": "query",
     "query": "SELECT [System.Id] FROM WorkItems WHERE [System.WorkItemType] = 'User Story' AND [System.State] = 'DOING'"
   }

### Simplified Helper Functions

1. Find items in multiple states:
   {
     "operation": "find_work_items",
     "states": "DOING,REVIEW"
   }

2. Find items without Epic parents:
   {
     "operation": "find_work_items",
     "states": "DOING,REVIEW",
     "has_parent": "false",
     "parent_type": "Epic"
   }

3. Get work item details:
   {
     "operation": "get_details",
     "ids": "123,456,789"
   }

4. Get list of fields:
   {
     "operation": "list_fields"
   }

5. Check if a work item has a parent Epic:
   {
     "operation": "get_related_work_items",
     "id": 123,
     "relation_type": "parent"
   }

### New Helper Operations

1. Search for work items by text:
   {
     "operation": "search",
     "search_text": "authentication issue"
   }

2. Search with filtering and JSON response:
   {
     "operation": "search",
     "search_text": "login problem",
     "states": "DOING,REVIEW",
     "format": "json"
   }

3. Get all available states:
   {
     "operation": "get_states"
   }

4. Get all work item types:
   {
     "operation": "get_work_item_types"
   }

5. Find orphaned items:
   {
     "operation": "find_orphaned_items"
   }

6. Find blocked items:
   {
     "operation": "find_blocked_items",
     "states": "DOING,In Progress"
   }

7. Find overdue items:
   {
     "operation": "find_overdue_items"
   }

8. Find items in current sprint:
   {
     "operation": "find_sprint_items"
   }

9. Pagination example:
   {
     "operation": "search",
     "search_text": "bug",
     "page": 2,
     "page_size": 10
   }

### Creating Different Work Item Types

1. Create a Task:
   {
     "operation": "create",
     "type": "Task",
     "title": "Implement login feature",
     "description": "Add authentication functionality to the login page"
   }

2. Create a User Story:
   {
     "operation": "create",
     "type": "User Story",
     "title": "As a user, I want to log in securely",
     "description": "Implement secure login functionality with proper error handling"
   }

3. Create an Epic:
   {
     "operation": "create",
     "type": "Epic",
     "title": "Authentication System Overhaul",
     "description": "Redesign the entire authentication system for better security and user experience"
   }

4. Create a Bug:
   {
     "operation": "create",
     "type": "Bug",
     "title": "Login page crashes on mobile",
     "description": "The login page is not rendering properly on mobile devices and crashes the app"
   }
`
	return mcp.NewToolResultText(examples), nil
}

// Helper function to list available fields without requiring a work item ID
func (tool *WorkItemTool) handleListFields(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get the list of fields using a direct REST API call since the client doesn't expose GetFields
	baseURL := fmt.Sprintf("%s/%s/_apis/wit/fields",
		tool.config.OrganizationURL,
		tool.config.Project)

	queryParams := url.Values{}
	queryParams.Add("api-version", "7.2-preview.2")

	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.SetBasicAuth("", tool.config.PersonalAccessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get fields: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get fields. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var fieldsResponse struct {
		Value []struct {
			Name          string `json:"name"`
			ReferenceName string `json:"referenceName"`
			Type          string `json:"type"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fieldsResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check if filter was provided
	var filter string
	if filterVal, ok := request.Params.Arguments["filter"].(string); ok {
		filter = strings.ToLower(filterVal)
	}

	var results []string
	for _, field := range fieldsResponse.Value {
		// Apply filter if provided
		if filter != "" {
			nameMatch := strings.Contains(strings.ToLower(field.Name), filter)
			refNameMatch := strings.Contains(strings.ToLower(field.ReferenceName), filter)
			if !nameMatch && !refNameMatch {
				continue
			}
		}

		result := fmt.Sprintf("Name: %s\nReference Name: %s\nType: %s\n---\n",
			field.Name,
			field.ReferenceName,
			field.Type)
		results = append(results, result)
	}

	if len(results) == 0 {
		if filter != "" {
			return mcp.NewToolResultText(fmt.Sprintf("No fields found matching '%s'", filter)), nil
		}
		return mcp.NewToolResultText("No fields found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Helper function to find work items by state and parent relationship
func (tool *WorkItemTool) handleFindWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get states parameter (required)
	statesStr, err := getStringArg(request, "states")
	if err != nil {
		return mcp.NewToolResultError("Required parameter 'states' is missing. Provide a comma-separated list of states."), nil
	}

	// Parse states
	states := strings.Split(statesStr, ",")
	for i, state := range states {
		states[i] = strings.TrimSpace(state)
	}

	// Build state condition for WIQL
	var stateCondition string
	if len(states) == 1 {
		stateCondition = fmt.Sprintf("[System.State] = '%s'", states[0])
	} else {
		stateValues := make([]string, len(states))
		for i, state := range states {
			stateValues[i] = fmt.Sprintf("'%s'", state)
		}
		stateCondition = fmt.Sprintf("[System.State] IN (%s)", strings.Join(stateValues, ", "))
	}

	// Check if we need to filter by parent relationship
	var parentCondition string
	if hasParentStr, ok := request.Params.Arguments["has_parent"].(string); ok {
		hasParent := strings.ToLower(hasParentStr) == "true"
		parentType := "Epic" // Default parent type

		if parentTypeStr, ok := request.Params.Arguments["parent_type"].(string); ok && parentTypeStr != "" {
			parentType = parentTypeStr
		}

		if hasParent {
			// Find items WITH parents
			parentCondition = fmt.Sprintf(" AND [System.Id] IN (SELECT [System.Id] FROM WorkItemLinks WHERE [Target].[System.WorkItemType] = '%s' AND [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse' MODE (MustContain))", parentType)
		} else {
			// Find items WITHOUT parents
			parentCondition = fmt.Sprintf(" AND [System.Id] NOT IN (SELECT [System.Id] FROM WorkItemLinks WHERE [Target].[System.WorkItemType] = '%s' AND [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse' MODE (MustContain))", parentType)
		}
	}

	// Get limit if provided
	limit := 100 // Default limit
	if limitVal, ok := request.Params.Arguments["limit"].(float64); ok {
		limit = int(limitVal)
	}

	// Build complete WIQL query
	query := fmt.Sprintf("SELECT TOP %d [System.Id], [System.Title], [System.State], [System.WorkItemType] FROM WorkItems WHERE %s%s ORDER BY [System.Id]",
		limit,
		stateCondition,
		parentCondition)

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Try to provide a more helpful error message with suggestions
		errorMsg := err.Error()
		var suggestion string

		if strings.Contains(errorMsg, "TF51005") {
			suggestion = "The search may be using invalid syntax. Try simplifying your search term."
		} else if strings.Contains(errorMsg, "TF51004") {
			suggestion = "Check for special characters in your search text that might need escaping."
		}

		errorResponse := fmt.Sprintf("Failed to search work items: %v\n\nYour search: %s", err, query)
		if suggestion != "" {
			errorResponse += "\n\nSuggestion: " + suggestion
		}

		return mcp.NewToolResultError(errorResponse), nil
	}

	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No work items found matching the criteria.\nStates: %s\nWIQL Query: %s",
			statesStr, query)), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return handleError(err, fmt.Sprintf("Failed to get work item details for search: %s", query)), nil
	}

	// Format results
	var results []string
	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Check for parent links
		hasParent := false
		parentInfo := "No parent"

		if item.Relations != nil {
			for _, relation := range *item.Relations {
				if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
					hasParent = true
					parentID, _ := extractWorkItemIDFromURL(*relation.Url)
					parentInfo = fmt.Sprintf("Has parent (ID: %d)", parentID)
					break
				}
			}
		}

		// Use hasParent status in the result output
		var parentStatus string
		if hasParent {
			parentStatus = "Yes"
		} else {
			parentStatus = "No"
		}

		result := fmt.Sprintf("ID: %d\nTitle: %s\nType: %s\nState: %s\nHas Parent: %s\nParent Info: %s\n---\n",
			id, title, workItemType, state, parentStatus, parentInfo)
		results = append(results, result)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleGetStates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get the work item states from the Azure DevOps API
	baseURL := fmt.Sprintf("%s/%s/_apis/wit/workitemtypes",
		tool.config.OrganizationURL,
		tool.config.Project)

	queryParams := url.Values{}
	queryParams.Add("api-version", "7.2-preview")

	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.SetBasicAuth("", tool.config.PersonalAccessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item types: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item types. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var typesResponse struct {
		Value []struct {
			Name          string `json:"name"`
			ReferenceName string `json:"referenceName"`
			States        []struct {
				Name     string `json:"name"`
				Color    string `json:"color"`
				Category string `json:"category"`
			} `json:"states"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&typesResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Organize all states
	statesByType := make(map[string][]string)
	allStates := make(map[string]bool)

	for _, typeInfo := range typesResponse.Value {
		var stateNames []string
		for _, state := range typeInfo.States {
			stateNames = append(stateNames, state.Name)
			allStates[state.Name] = true
		}
		statesByType[typeInfo.Name] = stateNames
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		// Return states in JSON format
		jsonResponse := map[string]any{
			"all_states":     mapKeysToSlice(allStates),
			"states_by_type": statesByType,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, "## All Available States\n")

	// Add all states first
	allStatesList := mapKeysToSlice(allStates)
	sort.Strings(allStatesList)
	results = append(results, strings.Join(allStatesList, ", "))

	// Then list states by work item type
	results = append(results, "\n\n## States by Work Item Type\n")

	// Sort work item types for consistent output
	typeNames := make([]string, 0, len(statesByType))
	for typeName := range statesByType {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		states := statesByType[typeName]
		results = append(results, fmt.Sprintf("\n### %s\n%s", typeName, strings.Join(states, ", ")))
	}

	// Add examples
	results = append(results, "\n\n## Examples\n")
	results = append(results, "To find work items in specific states:")
	results = append(results, fmt.Sprintf("```\n{\n  \"operation\": \"find_work_items\",\n  \"states\": \"%s\"\n}\n```", strings.Join(allStatesList[:min(3, len(allStatesList))], ",")))

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Helper function to convert map keys to a slice
func mapKeysToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// Helper function for min value (for Go versions before 1.21)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (tool *WorkItemTool) handleGetWorkItemTypes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get work item types from the Azure DevOps API
	baseURL := fmt.Sprintf("%s/%s/_apis/wit/workitemtypes",
		tool.config.OrganizationURL,
		tool.config.Project)

	queryParams := url.Values{}
	queryParams.Add("api-version", "7.2-preview")

	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.SetBasicAuth("", tool.config.PersonalAccessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item types: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item types. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var typesResponse struct {
		Value []struct {
			Name          string `json:"name"`
			ReferenceName string `json:"referenceName"`
			Description   string `json:"description"`
			Color         string `json:"color"`
			Icon          string `json:"icon"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&typesResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonData, err := json.MarshalIndent(typesResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, "## Available Work Item Types\n")

	for _, typeInfo := range typesResponse.Value {
		// Format type information
		description := typeInfo.Description
		if description == "" {
			description = "No description available"
		}

		results = append(results, fmt.Sprintf("### %s\n", typeInfo.Name))
		results = append(results, fmt.Sprintf("Reference Name: %s", typeInfo.ReferenceName))
		results = append(results, fmt.Sprintf("Description: %s", description))
		results = append(results, "---")
	}

	// Add examples section
	results = append(results, "\n## Examples\n")
	results = append(results, "To create a new work item of a specific type:")

	// Get the first type as an example (usually Task or User Story)
	exampleType := "Task"
	if len(typesResponse.Value) > 0 {
		exampleType = typesResponse.Value[0].Name
	}

	example := fmt.Sprintf(`{
  "operation": "create",
  "type": "%s",
  "title": "New %s",
  "description": "This is a new %s created via MCP"
}`, exampleType, exampleType, exampleType)

	results = append(results, "```\n"+example+"\n```")

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleSearchWorkItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get the search text
	searchText, err := getStringArg(request, "search_text")
	if err != nil {
		return mcp.NewToolResultError(`
Missing "search_text" parameter. Please provide the text to search for.

Example: "search_text": "authentication issue"
`), nil
	}

	// Get optional parameters
	// Get states parameter (optional)
	var statesCondition string
	if statesStr, ok := request.Params.Arguments["states"].(string); ok && statesStr != "" {
		states := strings.Split(statesStr, ",")
		for i, state := range states {
			states[i] = strings.TrimSpace(state)
		}

		if len(states) == 1 {
			statesCondition = fmt.Sprintf(" AND [System.State] = '%s'", states[0])
		} else {
			stateValues := make([]string, len(states))
			for i, state := range states {
				stateValues[i] = fmt.Sprintf("'%s'", state)
			}
			statesCondition = fmt.Sprintf(" AND [System.State] IN (%s)", strings.Join(stateValues, ", "))
		}
	}

	// Get work item types parameter (optional)
	var typesCondition string
	if typesStr, ok := request.Params.Arguments["types"].(string); ok && typesStr != "" {
		types := strings.Split(typesStr, ",")
		for i, t := range types {
			types[i] = strings.TrimSpace(t)
		}

		if len(types) == 1 {
			typesCondition = fmt.Sprintf(" AND [System.WorkItemType] = '%s'", types[0])
		} else {
			typeValues := make([]string, len(types))
			for i, t := range types {
				typeValues[i] = fmt.Sprintf("'%s'", t)
			}
			typesCondition = fmt.Sprintf(" AND [System.WorkItemType] IN (%s)", strings.Join(typeValues, ", "))
		}
	}

	// Build the WIQL query to search in title and description
	// Note: Azure DevOps doesn't support full-text search in WIQL directly,
	// so we use CONTAINS which has limitations but works for simple searches
	query := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType] 
FROM WorkItems 
WHERE (
  CONTAINS([System.Title], '%s') 
  OR CONTAINS([System.Description], '%s')
)%s%s
ORDER BY [System.ChangedDate] DESC
`, searchText, searchText, statesCondition, typesCondition)

	// Handle paging
	pageSize := 20 // Default page size
	page := 1      // Default page number

	if pageSizeVal, ok := request.Params.Arguments["page_size"].(float64); ok {
		pageSize = int(pageSizeVal)
	}

	if pageVal, ok := request.Params.Arguments["page"].(float64); ok {
		page = int(pageVal)
	}

	// Add paging to the query if supported by Azure DevOps version
	if pageSize > 0 {
		query = fmt.Sprintf("SELECT TOP %d %s", pageSize, query[7:])
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Special handling for query syntax errors
		errorMsg := err.Error()
		var suggestion string

		if strings.Contains(errorMsg, "TF51005") {
			suggestion = "The search may be using invalid syntax. Try simplifying your search term."
		} else if strings.Contains(errorMsg, "TF51004") {
			suggestion = "Check for special characters in your search text that might need escaping."
		}

		errorResponse := fmt.Sprintf("Failed to search work items: %v\n\nYour search: %s", err, searchText)
		if suggestion != "" {
			errorResponse += "\n\nSuggestion: " + suggestion
		}

		return mcp.NewToolResultError(errorResponse), nil
	}

	// If no items found, return a helpful message
	if len(*queryResult.WorkItems) == 0 {
		noResultsMsg := fmt.Sprintf("No work items found matching '%s'", searchText)

		// Add suggestions for no results
		noResultsMsg += "\n\nSuggestions:"
		noResultsMsg += "\n- Try using fewer or different keywords"
		noResultsMsg += "\n- Check for typos in your search term"
		if statesCondition != "" {
			noResultsMsg += "\n- Try searching without state filters"
		}
		if typesCondition != "" {
			noResultsMsg += "\n- Try searching across all work item types"
		}

		return mcp.NewToolResultText(noResultsMsg), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return handleError(err, fmt.Sprintf("Failed to get work item details for search: %s", searchText)), nil
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonResults := []map[string]any{}

		for _, item := range *workItems {
			fields := *item.Fields
			result := map[string]any{
				"id":    *item.Id,
				"title": fields["System.Title"],
				"state": fields["System.State"],
				"type":  fields["System.WorkItemType"],
				"url":   fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
			}

			// Add other important fields if they exist
			if description, ok := fields["System.Description"].(string); ok {
				result["description"] = description
			}

			if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
				if displayName, ok := assignedTo["displayName"].(string); ok {
					result["assigned_to"] = displayName
				}
			}

			// Check for parent relations
			if item.Relations != nil {
				parentIDs := []int{}
				for _, relation := range *item.Relations {
					if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
						parentID, _ := extractWorkItemIDFromURL(*relation.Url)
						parentIDs = append(parentIDs, parentID)
					}
				}

				if len(parentIDs) > 0 {
					result["parent_ids"] = parentIDs
				}
			}

			jsonResults = append(jsonResults, result)
		}

		// Create response with metadata
		jsonResponse := map[string]any{
			"search_text":   searchText,
			"total_results": len(jsonResults),
			"page":          page,
			"page_size":     pageSize,
			"results":       jsonResults,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, fmt.Sprintf("## Search Results for '%s'\n", searchText))
	results = append(results, fmt.Sprintf("Found %d matching work items\n", len(*workItems)))

	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Format the result
		result := fmt.Sprintf("### [%d] %s\n", id, title)
		result += fmt.Sprintf("Type: %s | State: %s\n", workItemType, state)

		// Add assignee if available
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				result += fmt.Sprintf("Assigned to: %s\n", displayName)
			}
		}

		// Add snippet of description if available
		if description, ok := fields["System.Description"].(string); ok {
			// Clean up HTML if present
			description = strings.ReplaceAll(description, "<div>", "")
			description = strings.ReplaceAll(description, "</div>", "")
			description = strings.ReplaceAll(description, "<br>", "\n")
			description = strings.ReplaceAll(description, "<br/>", "\n")
			description = strings.ReplaceAll(description, "<br />", "\n")

			// Truncate description for preview
			maxLen := 150
			if len(description) > maxLen {
				description = description[:maxLen] + "..."
			}

			result += fmt.Sprintf("Description: %s\n", description)
		}

		// Add URL to the work item
		result += fmt.Sprintf("URL: %s/_workitems/edit/%d\n", tool.config.OrganizationURL, id)

		// Add separator
		result += "---\n"

		results = append(results, result)
	}

	// Add pagination info and navigation help
	if page > 1 || len(*workItems) == pageSize {
		paginationInfo := fmt.Sprintf("\n## Pagination\nPage %d, %d items per page\n\n", page, pageSize)

		if page > 1 {
			paginationInfo += fmt.Sprintf("Previous page: `{\"operation\": \"search\", \"search_text\": \"%s\", \"page\": %d, \"page_size\": %d}`\n\n",
				searchText, page-1, pageSize)
		}

		if len(*workItems) == pageSize {
			paginationInfo += fmt.Sprintf("Next page: `{\"operation\": \"search\", \"search_text\": \"%s\", \"page\": %d, \"page_size\": %d}`",
				searchText, page+1, pageSize)
		}

		results = append(results, paginationInfo)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleFindOrphanedItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get optional parameters with defaults
	workItemTypes := "Task,Bug,User Story"
	states := "Active,New,DOING,REVIEW,In Progress,To Do"

	// Override defaults if provided
	if typesStr, ok := request.Params.Arguments["types"].(string); ok && typesStr != "" {
		workItemTypes = typesStr
	}

	if statesStr, ok := request.Params.Arguments["states"].(string); ok && statesStr != "" {
		states = statesStr
	}

	// Parse types and states
	types := strings.Split(workItemTypes, ",")
	for i, t := range types {
		types[i] = strings.TrimSpace(t)
	}

	statesList := strings.Split(states, ",")
	for i, s := range statesList {
		statesList[i] = strings.TrimSpace(s)
	}

	// Build the WIQL query to find orphaned items
	// An orphaned item is one that has no parent in the hierarchy
	query := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo]
FROM WorkItems
WHERE [System.WorkItemType] IN (%s)
AND [System.State] IN (%s)
AND [System.Id] NOT IN (
    SELECT [System.Id]
    FROM WorkItemLinks
    WHERE [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse'
    MODE (MustContain)
)
ORDER BY [System.ChangedDate] DESC
`, formatStringList(types), formatStringList(statesList))

	// Handle paging
	pageSize := 20 // Default page size
	page := 1      // Default page number

	if pageSizeVal, ok := request.Params.Arguments["page_size"].(float64); ok {
		pageSize = int(pageSizeVal)
	}

	if pageVal, ok := request.Params.Arguments["page"].(float64); ok {
		page = int(pageVal)
	}

	// Add paging to the query
	if pageSize > 0 {
		query = fmt.Sprintf("SELECT TOP %d %s", pageSize, query[7:])
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return handleError(err, "Failed to find orphaned work items"), nil
	}

	// If no items found, return a helpful message
	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText("No orphaned work items found with the specified criteria.\n\nA work item is considered 'orphaned' when it has no parent in the hierarchy."), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return handleError(err, "Failed to get orphaned work item details"), nil
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonResults := []map[string]any{}

		for _, item := range *workItems {
			fields := *item.Fields
			result := map[string]any{
				"id":    *item.Id,
				"title": fields["System.Title"],
				"state": fields["System.State"],
				"type":  fields["System.WorkItemType"],
				"url":   fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
			}

			// Add assignee if available
			if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
				if displayName, ok := assignedTo["displayName"].(string); ok {
					result["assigned_to"] = displayName
				}
			}

			jsonResults = append(jsonResults, result)
		}

		// Create response with metadata
		jsonResponse := map[string]any{
			"total_results": len(jsonResults),
			"page":          page,
			"page_size":     pageSize,
			"results":       jsonResults,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, "## Orphaned Work Items\n")
	results = append(results, fmt.Sprintf("Found %d orphaned work items\n", len(*workItems)))
	results = append(results, "These work items have no parent in the hierarchy.\n")

	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Format the result
		result := fmt.Sprintf("### [%d] %s\n", id, title)
		result += fmt.Sprintf("Type: %s | State: %s\n", workItemType, state)

		// Add assignee if available
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				result += fmt.Sprintf("Assigned to: %s\n", displayName)
			}
		}

		// Add URL to the work item
		result += fmt.Sprintf("URL: %s/_workitems/edit/%d\n", tool.config.OrganizationURL, id)

		// Add separator
		result += "---\n"

		results = append(results, result)
	}

	// Add pagination info if needed
	if page > 1 || len(*workItems) == pageSize {
		paginationInfo := fmt.Sprintf("\n## Pagination\nPage %d, %d items per page\n\n", page, pageSize)

		if page > 1 {
			paginationInfo += fmt.Sprintf("Previous page: `{\"operation\": \"find_orphaned_items\", \"page\": %d, \"page_size\": %d}`\n\n",
				page-1, pageSize)
		}

		if len(*workItems) == pageSize {
			paginationInfo += fmt.Sprintf("Next page: `{\"operation\": \"find_orphaned_items\", \"page\": %d, \"page_size\": %d}`",
				page+1, pageSize)
		}

		results = append(results, paginationInfo)
	}

	// Add helpful suggestions
	results = append(results, "\n## What to do with orphaned items\n")
	results = append(results, "Orphaned items should generally be linked to a parent work item for proper organization.")
	results = append(results, "To link an item to a parent, use the following command:")
	results = append(results, "```\n{\n  \"operation\": \"manage_relations\",\n  \"source_id\": \"ITEM_ID\",\n  \"target_id\": \"PARENT_ID\",\n  \"relation_type\": \"parent\",\n  \"operation\": \"add\"\n}\n```")

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleFindBlockedItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get optional parameters with defaults
	states := "Active,DOING,In Progress,REVIEW"

	// Override defaults if provided
	if statesStr, ok := request.Params.Arguments["states"].(string); ok && statesStr != "" {
		states = statesStr
	}

	// Parse states
	statesList := strings.Split(states, ",")
	for i, s := range statesList {
		statesList[i] = strings.TrimSpace(s)
	}

	// Build the WIQL query to find blocked items
	// A blocked item is one that has the "Blocked" field set to "Yes" or has a tag containing "blocked"
	query := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo], [System.Tags]
FROM WorkItems
WHERE [System.State] IN (%s)
AND (
    [Microsoft.VSTS.CMMI.Blocked] = 'Yes'
    OR CONTAINS([System.Tags], 'blocked')
    OR CONTAINS([System.Title], 'blocked')
    OR CONTAINS([System.Description], 'blocked')
)
ORDER BY [System.ChangedDate] DESC
`, formatStringList(statesList))

	// Handle paging
	pageSize := 20 // Default page size
	page := 1      // Default page number

	if pageSizeVal, ok := request.Params.Arguments["page_size"].(float64); ok {
		pageSize = int(pageSizeVal)
	}

	if pageVal, ok := request.Params.Arguments["page"].(float64); ok {
		page = int(pageVal)
	}

	// Add paging to the query
	if pageSize > 0 {
		query = fmt.Sprintf("SELECT TOP %d %s", pageSize, query[7:])
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Handle the case where the Blocked field might not exist
		if strings.Contains(err.Error(), "Microsoft.VSTS.CMMI.Blocked") {
			// Try an alternative query without the Blocked field
			query = strings.ReplaceAll(query, "[Microsoft.VSTS.CMMI.Blocked] = 'Yes'\n    OR ", "")

			wiqlArgs.Wiql.Query = &query
			queryResult, err = tool.client.QueryByWiql(ctx, wiqlArgs)
			if err != nil {
				return handleError(err, "Failed to find blocked work items"), nil
			}
		} else {
			return handleError(err, "Failed to find blocked work items"), nil
		}
	}

	// If no items found, return a helpful message
	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText("No blocked work items found with the specified criteria.\n\nA work item is considered 'blocked' when it has the Blocked field set to Yes, or has a tag containing 'blocked', or has 'blocked' in the title or description."), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return handleError(err, "Failed to get blocked work item details"), nil
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonResults := []map[string]any{}

		for _, item := range *workItems {
			fields := *item.Fields
			result := map[string]any{
				"id":    *item.Id,
				"title": fields["System.Title"],
				"state": fields["System.State"],
				"type":  fields["System.WorkItemType"],
				"url":   fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
			}

			// Add tags if available
			if tags, ok := fields["System.Tags"].(string); ok {
				result["tags"] = tags
			}

			// Add assignee if available
			if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
				if displayName, ok := assignedTo["displayName"].(string); ok {
					result["assigned_to"] = displayName
				}
			}

			jsonResults = append(jsonResults, result)
		}

		// Create response with metadata
		jsonResponse := map[string]any{
			"total_results": len(jsonResults),
			"page":          page,
			"page_size":     pageSize,
			"results":       jsonResults,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, "## Blocked Work Items\n")
	results = append(results, fmt.Sprintf("Found %d blocked work items\n", len(*workItems)))
	results = append(results, "These work items are marked as blocked or have 'blocked' in their title, description, or tags.\n")

	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Format the result
		result := fmt.Sprintf("### [%d] %s\n", id, title)
		result += fmt.Sprintf("Type: %s | State: %s\n", workItemType, state)

		// Add tags if available
		if tags, ok := fields["System.Tags"].(string); ok && tags != "" {
			result += fmt.Sprintf("Tags: %s\n", tags)
		}

		// Add assignee if available
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				result += fmt.Sprintf("Assigned to: %s\n", displayName)
			}
		}

		// Add URL to the work item
		result += fmt.Sprintf("URL: %s/_workitems/edit/%d\n", tool.config.OrganizationURL, id)

		// Add separator
		result += "---\n"

		results = append(results, result)
	}

	// Add pagination info if needed
	if page > 1 || len(*workItems) == pageSize {
		paginationInfo := fmt.Sprintf("\n## Pagination\nPage %d, %d items per page\n\n", page, pageSize)

		if page > 1 {
			paginationInfo += fmt.Sprintf("Previous page: `{\"operation\": \"find_blocked_items\", \"page\": %d, \"page_size\": %d}`\n\n",
				page-1, pageSize)
		}

		if len(*workItems) == pageSize {
			paginationInfo += fmt.Sprintf("Next page: `{\"operation\": \"find_blocked_items\", \"page\": %d, \"page_size\": %d}`",
				page+1, pageSize)
		}

		results = append(results, paginationInfo)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func (tool *WorkItemTool) handleFindOverdueItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get optional parameters with defaults
	states := "Active,DOING,In Progress,REVIEW"

	// Override defaults if provided
	if statesStr, ok := request.Params.Arguments["states"].(string); ok && statesStr != "" {
		states = statesStr
	}

	// Parse states
	statesList := strings.Split(states, ",")
	for i, s := range statesList {
		statesList[i] = strings.TrimSpace(s)
	}

	// Build the WIQL query to find overdue items
	// An overdue item is one that has a due date in the past
	query := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo], [Microsoft.VSTS.Scheduling.DueDate]
FROM WorkItems
WHERE [System.State] IN (%s)
AND [Microsoft.VSTS.Scheduling.DueDate] < @Today
ORDER BY [Microsoft.VSTS.Scheduling.DueDate] ASC
`, formatStringList(statesList))

	// Handle paging
	pageSize := 20 // Default page size
	page := 1      // Default page number

	if pageSizeVal, ok := request.Params.Arguments["page_size"].(float64); ok {
		pageSize = int(pageSizeVal)
	}

	if pageVal, ok := request.Params.Arguments["page"].(float64); ok {
		page = int(pageVal)
	}

	// Add paging to the query
	if pageSize > 0 {
		query = fmt.Sprintf("SELECT TOP %d %s", pageSize, query[7:])
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Handle the case where the DueDate field might not exist or have a different name
		if strings.Contains(err.Error(), "Microsoft.VSTS.Scheduling.DueDate") {
			// Try an alternative query with a different due date field
			alternativeQuery := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo]
FROM WorkItems
WHERE [System.State] IN (%s)
AND [System.CreatedDate] < @Today-14
ORDER BY [System.CreatedDate] ASC
`, formatStringList(statesList))

			if pageSize > 0 {
				alternativeQuery = fmt.Sprintf("SELECT TOP %d %s", pageSize, alternativeQuery[7:])
			}

			wiqlArgs.Wiql.Query = &alternativeQuery
			queryResult, err = tool.client.QueryByWiql(ctx, wiqlArgs)
			if err != nil {
				return handleError(err, "Failed to find overdue work items"), nil
			}
		} else {
			return handleError(err, "Failed to find overdue work items"), nil
		}
	}

	// If no items found, return a helpful message
	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText("No overdue work items found with the specified criteria.\n\nA work item is considered 'overdue' when it has a due date in the past."), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return handleError(err, "Failed to get overdue work item details"), nil
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonResults := []map[string]any{}

		for _, item := range *workItems {
			fields := *item.Fields
			result := map[string]any{
				"id":    *item.Id,
				"title": fields["System.Title"],
				"state": fields["System.State"],
				"type":  fields["System.WorkItemType"],
				"url":   fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
			}

			// Add due date if available
			if dueDate, ok := fields["Microsoft.VSTS.Scheduling.DueDate"].(string); ok {
				result["due_date"] = dueDate
			}

			// Add assignee if available
			if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
				if displayName, ok := assignedTo["displayName"].(string); ok {
					result["assigned_to"] = displayName
				}
			}

			jsonResults = append(jsonResults, result)
		}

		// Create response with metadata
		jsonResponse := map[string]any{
			"total_results": len(jsonResults),
			"page":          page,
			"page_size":     pageSize,
			"results":       jsonResults,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, "## Overdue Work Items\n")
	results = append(results, fmt.Sprintf("Found %d overdue work items\n", len(*workItems)))
	results = append(results, "These work items have due dates in the past.\n")

	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Format the result
		result := fmt.Sprintf("### [%d] %s\n", id, title)
		result += fmt.Sprintf("Type: %s | State: %s\n", workItemType, state)

		// Add due date if available
		if dueDate, ok := fields["Microsoft.VSTS.Scheduling.DueDate"].(string); ok {
			result += fmt.Sprintf("Due Date: %s\n", dueDate)
		}

		// Add assignee if available
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				result += fmt.Sprintf("Assigned to: %s\n", displayName)
			}
		}

		// Add URL to the work item
		result += fmt.Sprintf("URL: %s/_workitems/edit/%d\n", tool.config.OrganizationURL, id)

		// Add separator
		result += "---\n"

		results = append(results, result)
	}

	// Add pagination info if needed
	if page > 1 || len(*workItems) == pageSize {
		paginationInfo := fmt.Sprintf("\n## Pagination\nPage %d, %d items per page\n\n", page, pageSize)

		if page > 1 {
			paginationInfo += fmt.Sprintf("Previous page: `{\"operation\": \"find_overdue_items\", \"page\": %d, \"page_size\": %d}`\n\n",
				page-1, pageSize)
		}

		if len(*workItems) == pageSize {
			paginationInfo += fmt.Sprintf("Next page: `{\"operation\": \"find_overdue_items\", \"page\": %d, \"page_size\": %d}`",
				page+1, pageSize)
		}

		results = append(results, paginationInfo)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Helper function to format a list of strings for WIQL queries
func formatStringList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("'%s'", item)
	}
	return strings.Join(quoted, ", ")
}

func (tool *WorkItemTool) handleFindSprintItems(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get current sprint information first
	baseURL := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations",
		tool.config.OrganizationURL,
		tool.config.Project)

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
	req.SetBasicAuth("", tool.config.PersonalAccessToken)

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
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Path      string    `json:"path"`
			StartDate time.Time `json:"startDate"`
			EndDate   time.Time `json:"finishDate"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&sprintResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse sprint response: %v", err)), nil
	}

	if len(sprintResponse.Value) == 0 {
		return mcp.NewToolResultText("No active sprint found."), nil
	}

	currentSprint := sprintResponse.Value[0]

	// Get optional parameters
	var workItemTypes string
	if typesStr, ok := request.Params.Arguments["types"].(string); ok && typesStr != "" {
		workItemTypes = typesStr
	} else {
		// Default to common work item types
		workItemTypes = "Task,Bug,User Story,Epic"
	}

	var states string
	if statesStr, ok := request.Params.Arguments["states"].(string); ok && statesStr != "" {
		states = statesStr
	} else {
		// Default to active states
		states = "Active,DOING,New,In Progress,REVIEW"
	}

	// Parse types and states
	types := strings.Split(workItemTypes, ",")
	for i, t := range types {
		types[i] = strings.TrimSpace(t)
	}

	statesList := strings.Split(states, ",")
	for i, s := range statesList {
		statesList[i] = strings.TrimSpace(s)
	}

	// Handle paging
	pageSize := 50 // Default page size
	page := 1      // Default page number

	if pageSizeVal, ok := request.Params.Arguments["page_size"].(float64); ok {
		pageSize = int(pageSizeVal)
	}

	if pageVal, ok := request.Params.Arguments["page"].(float64); ok {
		page = int(pageVal)
	}

	// Build the WIQL query to find items in the current sprint/iteration
	query := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo]
FROM WorkItems
WHERE [System.WorkItemType] IN (%s)
AND [System.State] IN (%s)
AND [System.IterationPath] UNDER '%s'
ORDER BY [System.ChangedDate] DESC
`, formatStringList(types), formatStringList(statesList), currentSprint.Path)

	// Add paging to the query
	if pageSize > 0 {
		query = fmt.Sprintf("SELECT TOP %d %s", pageSize, query[7:])
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Try a simpler query if this fails - might be an issue with the iteration path format
		alternativeQuery := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo]
FROM WorkItems
WHERE [System.WorkItemType] IN (%s)
AND [System.State] IN (%s)
AND [System.IterationPath] CONTAINS '%s'
ORDER BY [System.ChangedDate] DESC
`, formatStringList(types), formatStringList(statesList), currentSprint.Name)

		if pageSize > 0 {
			alternativeQuery = fmt.Sprintf("SELECT TOP %d %s", pageSize, alternativeQuery[7:])
		}

		wiqlArgs.Wiql.Query = &alternativeQuery
		queryResult, err = tool.client.QueryByWiql(ctx, wiqlArgs)
		if err != nil {
			return handleError(err, fmt.Sprintf("Failed to find work items in current sprint (%s)", currentSprint.Name)), nil
		}
	}

	// If no items found, return a helpful message
	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No work items found in the current sprint: %s (%s to %s).",
			currentSprint.Name,
			currentSprint.StartDate.Format("2006-01-02"),
			currentSprint.EndDate.Format("2006-01-02"))), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return handleError(err, "Failed to get work item details"), nil
	}

	// Check if JSON format is requested
	format, _ := getStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonResults := []map[string]any{}

		for _, item := range *workItems {
			fields := *item.Fields
			result := map[string]any{
				"id":    *item.Id,
				"title": fields["System.Title"],
				"state": fields["System.State"],
				"type":  fields["System.WorkItemType"],
				"url":   fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
			}

			// Add assignee if available
			if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
				if displayName, ok := assignedTo["displayName"].(string); ok {
					result["assigned_to"] = displayName
				}
			}

			// Add iteration path if available
			if iterationPath, ok := fields["System.IterationPath"].(string); ok {
				result["iteration_path"] = iterationPath
			}

			// Check for parent relations
			if item.Relations != nil {
				parentIDs := []int{}
				for _, relation := range *item.Relations {
					if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
						parentID, _ := extractWorkItemIDFromURL(*relation.Url)
						parentIDs = append(parentIDs, parentID)
					}
				}

				if len(parentIDs) > 0 {
					result["parent_ids"] = parentIDs
				}
			}

			jsonResults = append(jsonResults, result)
		}

		// Create response with metadata
		jsonResponse := map[string]any{
			"sprint": map[string]any{
				"name":       currentSprint.Name,
				"start_date": currentSprint.StartDate.Format("2006-01-02"),
				"end_date":   currentSprint.EndDate.Format("2006-01-02"),
			},
			"total_results": len(jsonResults),
			"page":          page,
			"page_size":     pageSize,
			"results":       jsonResults,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, fmt.Sprintf("## Work Items in Current Sprint: %s\n", currentSprint.Name))
	results = append(results, fmt.Sprintf("Sprint Period: %s to %s\n",
		currentSprint.StartDate.Format("2006-01-02"),
		currentSprint.EndDate.Format("2006-01-02")))
	results = append(results, fmt.Sprintf("Found %d work items\n", len(*workItems)))

	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Format the result
		result := fmt.Sprintf("### [%d] %s\n", id, title)
		result += fmt.Sprintf("Type: %s | State: %s\n", workItemType, state)

		// Add assignee if available
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				result += fmt.Sprintf("Assigned to: %s\n", displayName)
			}
		}

		// Add iteration path if available
		if iterationPath, ok := fields["System.IterationPath"].(string); ok {
			result += fmt.Sprintf("Iteration: %s\n", iterationPath)
		}

		// Check for parent links
		if item.Relations != nil {
			var parentIDs []string
			for _, relation := range *item.Relations {
				if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
					parentID, _ := extractWorkItemIDFromURL(*relation.Url)
					parentIDs = append(parentIDs, fmt.Sprintf("%d", parentID))
				}
			}

			if len(parentIDs) > 0 {
				result += fmt.Sprintf("Parent IDs: %s\n", strings.Join(parentIDs, ", "))
			}
		}

		// Add URL to the work item
		result += fmt.Sprintf("URL: %s/_workitems/edit/%d\n", tool.config.OrganizationURL, id)

		// Add separator
		result += "---\n"

		results = append(results, result)
	}

	// Add pagination info if needed
	if page > 1 || len(*workItems) == pageSize {
		paginationInfo := fmt.Sprintf("\n## Pagination\nPage %d, %d items per page\n\n", page, pageSize)

		if page > 1 {
			paginationInfo += fmt.Sprintf("Previous page: `{\"operation\": \"find_sprint_items\", \"page\": %d, \"page_size\": %d}`\n\n",
				page-1, pageSize)
		}

		if len(*workItems) == pageSize {
			paginationInfo += fmt.Sprintf("Next page: `{\"operation\": \"find_sprint_items\", \"page\": %d, \"page_size\": %d}`",
				page+1, pageSize)
		}

		results = append(results, paginationInfo)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}
