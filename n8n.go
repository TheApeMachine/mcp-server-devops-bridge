package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Add N8N tools
func addN8NTools(s *server.MCPServer) {
	// Get Workflow List
	listWorkflowsTool := mcp.NewTool("n8n_list_workflows",
		mcp.WithDescription("List all N8N workflows"),
		mcp.WithBoolean("active_only",
			mcp.Description("Only show active workflows"),
		),
	)
	s.AddTool(listWorkflowsTool, handleListN8NWorkflows)

	// Get Workflow Details
	getWorkflowTool := mcp.NewTool("n8n_get_workflow",
		mcp.WithDescription("Get details of a specific N8N workflow"),
		mcp.WithString("workflow_id",
			mcp.Required(),
			mcp.Description("ID of the workflow to retrieve"),
		),
	)
	s.AddTool(getWorkflowTool, handleGetN8NWorkflow)

	// Toggle Workflow Active State
	toggleWorkflowTool := mcp.NewTool("n8n_toggle_workflow",
		mcp.WithDescription("Activate or deactivate an N8N workflow"),
		mcp.WithString("workflow_id",
			mcp.Required(),
			mcp.Description("ID of the workflow to toggle"),
		),
		mcp.WithBoolean("active",
			mcp.Required(),
			mcp.Description("True to activate, false to deactivate"),
		),
	)
	s.AddTool(toggleWorkflowTool, handleToggleN8NWorkflow)

	// Get Workflow Executions
	getExecutionsTool := mcp.NewTool("n8n_get_executions",
		mcp.WithDescription("Get execution history for a workflow"),
		mcp.WithString("workflow_id",
			mcp.Required(),
			mcp.Description("ID of the workflow"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of executions to retrieve"),
		),
		mcp.WithBoolean("error_only",
			mcp.Description("Only show failed executions"),
		),
	)
	s.AddTool(getExecutionsTool, handleGetN8NExecutions)

	// Create Workflow
	createWorkflowTool := mcp.NewTool("n8n_create_workflow",
		mcp.WithDescription("Create a new N8N workflow"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the workflow"),
		),
		mcp.WithString("nodes",
			mcp.Required(),
			mcp.Description("JSON string containing workflow nodes configuration"),
		),
		mcp.WithString("connections",
			mcp.Required(),
			mcp.Description("JSON string containing workflow connections configuration"),
		),
		mcp.WithBoolean("active",
			mcp.Description("Set to true to activate the workflow upon creation"),
		),
	)
	s.AddTool(createWorkflowTool, handleCreateN8NWorkflow)

	// Edit Workflow
	editWorkflowTool := mcp.NewTool("n8n_edit_workflow",
		mcp.WithDescription("Edit an existing N8N workflow"),
		mcp.WithString("workflow_id",
			mcp.Required(),
			mcp.Description("ID of the workflow to edit"),
		),
		mcp.WithString("name",
			mcp.Description("New name for the workflow"),
		),
		mcp.WithString("nodes",
			mcp.Description("JSON string containing updated workflow nodes configuration"),
		),
		mcp.WithString("connections",
			mcp.Description("JSON string containing updated workflow connections configuration"),
		),
		mcp.WithBoolean("active",
			mcp.Description("Update workflow active state"),
		),
	)
	s.AddTool(editWorkflowTool, handleEditN8NWorkflow)

	// Add N8N Workflow Format Prompt
	s.AddPrompt(mcp.NewPrompt("n8n_workflow_format",
		mcp.WithPromptDescription("Helper for formatting N8N workflow configurations"),
		mcp.WithArgument("workflow_type",
			mcp.ArgumentDescription("Type of workflow to format (webhook, scheduler, etc)"),
			mcp.RequiredArgument(),
		),
	), handleN8NWorkflowFormatPrompt)
}

// N8N Handlers
func handleListN8NWorkflows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	activeOnly, _ := request.Params.Arguments["active_only"].(bool)

	url := fmt.Sprintf("%s/api/v1/workflows", n8nBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get workflows: %v", err)), nil
	}
	defer resp.Body.Close()

	// Read the raw response for debugging
	var rawResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse raw response: %v", err)), nil
	}

	// Extract workflows from the response
	workflows, ok := rawResponse["data"].([]interface{})
	if !ok {
		return mcp.NewToolResultError("Unexpected response format"), nil
	}

	var results []string
	for _, wf := range workflows {
		workflow, ok := wf.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract fields with type checking
		id, _ := workflow["id"].(string)
		name, _ := workflow["name"].(string)
		active, _ := workflow["active"].(bool)
		createdAt, _ := workflow["createdAt"].(string)
		updatedAt, _ := workflow["updatedAt"].(string)

		if activeOnly && !active {
			continue
		}

		results = append(results, fmt.Sprintf("ID: %s\nName: %s\nActive: %v\nCreated: %s\nUpdated: %s\n---",
			id, name, active, createdAt, updatedAt))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No workflows found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func handleGetN8NWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID := request.Params.Arguments["workflow_id"].(string)

	if _, err := strconv.Atoi(workflowID); err != nil {
		return mcp.NewToolResultError("Invalid workflow ID format"), nil
	}

	url := fmt.Sprintf("%s/api/v1/workflows/%s", n8nBaseURL, workflowID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get workflow: %v", err)), nil
	}
	defer resp.Body.Close()

	var workflow struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Active      bool            `json:"active"`
		Nodes       json.RawMessage `json:"nodes"`
		Connections json.RawMessage `json:"connections"`
		Created     string          `json:"createdAt"`
		Updated     string          `json:"updatedAt"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	result := fmt.Sprintf("ID: %s\nName: %s\nActive: %v\nCreated: %s\nUpdated: %s\n\nNodes: %s\n\nConnections: %s",
		workflow.ID, workflow.Name, workflow.Active, workflow.Created, workflow.Updated,
		workflow.Nodes, workflow.Connections)

	return mcp.NewToolResultText(result), nil
}

func handleToggleN8NWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID := request.Params.Arguments["workflow_id"].(string)
	active := request.Params.Arguments["active"].(bool)

	url := fmt.Sprintf("%s/api/v1/workflows/%s/activate", n8nBaseURL, workflowID)
	method := "POST"
	if !active {
		method = "DELETE"
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to toggle workflow: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to toggle workflow. Status: %d", resp.StatusCode)), nil
	}

	status := "activated"
	if !active {
		status = "deactivated"
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully %s workflow %s", status, workflowID)), nil
}

func handleGetN8NExecutions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID := request.Params.Arguments["workflow_id"].(string)
	limit, hasLimit := request.Params.Arguments["limit"].(float64)
	errorOnly, _ := request.Params.Arguments["error_only"].(bool)

	url := fmt.Sprintf("%s/api/v1/executions?workflowId=%s", n8nBaseURL, workflowID)
	if hasLimit {
		url += fmt.Sprintf("&limit=%d", int(limit))
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get executions: %v", err)), nil
	}
	defer resp.Body.Close()

	var executions []struct {
		ID        string    `json:"id"`
		Status    string    `json:"status"`
		StartedAt time.Time `json:"startedAt"`
		Error     struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&executions); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	var results []string
	for _, exec := range executions {
		if errorOnly && exec.Status != "error" {
			continue
		}

		result := fmt.Sprintf("ID: %s\nStatus: %s\nStarted: %s",
			exec.ID, exec.Status, exec.StartedAt.Format(time.RFC3339))

		if exec.Status == "error" {
			result += fmt.Sprintf("\nError: %s", exec.Error.Message)
		}

		results = append(results, result+"\n---")
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No executions found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// Handler for creating N8N workflow
func handleCreateN8NWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.Params.Arguments["name"].(string)
	nodesJSON := request.Params.Arguments["nodes"].(string)
	connectionsJSON := request.Params.Arguments["connections"].(string)

	// Parse nodes and connections JSON
	var nodes json.RawMessage
	if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid nodes JSON: %v", err)), nil
	}

	var connections json.RawMessage
	if err := json.Unmarshal([]byte(connectionsJSON), &connections); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid connections JSON: %v", err)), nil
	}

	// Create workflow payload with the exact N8N API structure
	payload := map[string]interface{}{
		"name":        name,
		"nodes":       nodes,
		"connections": connections,
		"settings": map[string]interface{}{
			"saveExecutionProgress":    true,
			"saveManualExecutions":     true,
			"saveDataErrorExecution":   "all",
			"saveDataSuccessExecution": "all",
			"executionTimeout":         3600,
			"timezone":                 "UTC",
			"executionOrder":           "v1",
		},
		"staticData": map[string]interface{}{
			"lastId": nil,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create payload: %v", err)), nil
	}

	// Send request to N8N API
	url := fmt.Sprintf("%s/api/v1/workflows", n8nBaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create workflow: %v", err)), nil
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read response body: %v", err)), nil
	}

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create workflow. Status: %d\nResponse: %s",
			resp.StatusCode, string(body))), nil
	}

	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v\nResponse body: %s",
			err, string(body))), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created workflow with ID: %s", response.Data.ID)), nil
}

// Handler for editing N8N workflow
func handleEditN8NWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID := request.Params.Arguments["workflow_id"].(string)

	// Build update payload with only provided fields
	payload := make(map[string]interface{})

	if name, ok := request.Params.Arguments["name"].(string); ok {
		payload["name"] = name
	}

	if nodesJSON, ok := request.Params.Arguments["nodes"].(string); ok {
		var nodes json.RawMessage
		if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid nodes JSON: %v", err)), nil
		}
		payload["nodes"] = nodes
	}

	if connectionsJSON, ok := request.Params.Arguments["connections"].(string); ok {
		var connections json.RawMessage
		if err := json.Unmarshal([]byte(connectionsJSON), &connections); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid connections JSON: %v", err)), nil
		}
		payload["connections"] = connections
	}

	if active, ok := request.Params.Arguments["active"].(bool); ok {
		payload["active"] = active
	}

	// If no fields were provided to update
	if len(payload) == 0 {
		return mcp.NewToolResultError("No fields provided for update"), nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create payload: %v", err)), nil
	}

	// Send request to N8N API
	url := fmt.Sprintf("%s/api/v1/workflows/%s", n8nBaseURL, workflowID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("X-N8N-API-KEY", n8nAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update workflow: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update workflow. Status: %d", resp.StatusCode)), nil
	}

	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully updated workflow %s", workflowID)), nil
}

func handleN8NWorkflowFormatPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	workflowType := request.Params.Arguments["workflow_type"]

	return mcp.NewGetPromptResult(
		"N8N Workflow Format Helper",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				"system",
				mcp.NewTextContent(`You are an N8N workflow expert. Format workflow configurations according to N8N's API structure.
The workflow configuration must follow this exact structure:
{
    "name": "Workflow Name",
    "nodes": [...],
    "connections": {...},
    "settings": {
        "saveExecutionProgress": true,
        "saveManualExecutions": true,
        "saveDataErrorExecution": "all",
        "saveDataSuccessExecution": "all",
        "executionTimeout": 3600,
        "timezone": "UTC",
        "executionOrder": "v1"
    },
    "staticData": {
        "lastId": null
    }
}

Each node must include:
- Unique id (UUID v4)
- name
- type (e.g., "n8n-nodes-base.webhook")
- typeVersion
- position [x, y]
- parameters object
- Other required fields based on node type

Connections must specify the flow between nodes using their IDs.`),
			),
			mcp.NewPromptMessage(
				"assistant",
				mcp.NewTextContent(fmt.Sprintf("I'll help you create a %s workflow configuration that follows N8N's required structure. What functionality do you need?", workflowType)),
			),
		},
	), nil
}
