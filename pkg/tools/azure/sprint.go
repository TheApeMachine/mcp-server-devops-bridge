package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// SprintOutput defines the structure for a single sprint's details for output.
type SprintOutput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IterationPath string `json:"iteration_path"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
}

type SprintTool struct {
	handle mcp.Tool
	conn   *azuredevops.Connection
	config AzureDevOpsConfig
}

func (tool *SprintTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *SprintTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		op string
		ok bool
	)

	if op, ok = request.Params.Arguments["operation"].(string); !ok {
		return mcp.NewToolResultError("Missing operation parameter"), nil
	}

	switch op {
	case "get_current_sprint_details": // Renamed for clarity
		return tool.handleGetCurrentSprintDetails(ctx, request)
	case "list_sprints": // Renamed for clarity
		return tool.handleListSprints(ctx, request)
	}

	return mcp.NewToolResultError(fmt.Sprintf("Unsupported operation: %s", op)), nil
}

func NewSprintTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	return &SprintTool{
		handle: mcp.NewTool("sprint", // Tool name remains 'sprint'
			mcp.WithDescription("Get information about sprints (iterations) in Azure DevOps."),
			mcp.WithString("operation", mcp.Required(), mcp.Description("Operation to perform: get_current_sprint_details, list_sprints"), mcp.Enum("get_current_sprint_details", "list_sprints")),
			mcp.WithString("format", mcp.Description("Response format: 'text' (default) or 'json'")),
			mcp.WithString("include_completed", mcp.Description("For list_sprints: whether to include completed sprints (default: false)"), mcp.Enum("true", "false")),
		),
		conn:   conn,
		config: config,
	}
}

// APIIterationResponseValue defines the structure for parsing individual iteration from Azure DevOps API.
type APIIterationResponseValue struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"` // This is the Iteration Path
	Attributes struct {
		StartDate  *time.Time `json:"startDate"`  // Use pointer to handle potential nulls if API sends that
		FinishDate *time.Time `json:"finishDate"` // Use pointer
		TimeFrame  string     `json:"timeFrame"`
	} `json:"attributes"`
}

// APIIterationsResponse defines the overall structure for the iterations API response.
type APIIterationsResponse struct {
	Count int64                       `json:"count"`
	Value []APIIterationResponseValue `json:"value"`
}

func formatSprintOutputToJSON(sprintOutput SprintOutput) (string, error) {
	jsonData, err := json.MarshalIndent(sprintOutput, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func formatSprintOutputToText(sprintOutput SprintOutput) string {
	return fmt.Sprintf("Sprint ID: %s\nName: %s\nIteration Path: %s\nStart Date: %s\nEnd Date: %s",
		sprintOutput.ID, sprintOutput.Name, sprintOutput.IterationPath, sprintOutput.StartDate, sprintOutput.EndDate)
}

func formatSprintsOutputToJSON(sprintsOutput []SprintOutput) (string, error) {
	jsonData, err := json.MarshalIndent(sprintsOutput, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func formatSprintsOutputToText(sprintsOutput []SprintOutput) string {
	var results []string
	if len(sprintsOutput) == 0 {
		return "No sprints found matching the criteria."
	}
	results = append(results, "## Sprints List\n")
	for _, sprint := range sprintsOutput {
		results = append(results, fmt.Sprintf("Name: %s\n  ID: %s\n  Iteration Path: %s\n  Start Date: %s\n  End Date: %s\n---",
			sprint.Name, sprint.ID, sprint.IterationPath, sprint.StartDate, sprint.EndDate))
	}
	return strings.Join(results, "\n")
}

func (tool *SprintTool) callIterationsAPI(timeframe string) (*APIIterationsResponse, error) {
	teamID := "702276d6-e0ea-4d99-b224-e8d468c12d9d" // Hardcoded team ID as per user
	baseURL := fmt.Sprintf("%s/%s/%s/_apis/work/teamsettings/iterations",
		tool.config.OrganizationURL,
		tool.config.Project,
		teamID)

	queryParams := url.Values{}
	if timeframe != "" { // Allow fetching all if timeframe is empty, though API might default
		queryParams.Add("$timeframe", timeframe)
	}
	queryParams.Add("api-version", "7.1") // Using a generally available API version

	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.SetBasicAuth("", tool.config.PersonalAccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call iterations API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get iterations, API status: %d", resp.StatusCode)
	}

	var apiResponse APIIterationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse iterations API response: %v", err)
	}
	return &apiResponse, nil
}

func (tool *SprintTool) handleGetCurrentSprintDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiResponse, err := tool.callIterationsAPI("current")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(apiResponse.Value) == 0 {
		return mcp.NewToolResultText("No active sprint found."), nil
	}

	currentAPISprint := apiResponse.Value[0]
	sprintOutput := SprintOutput{
		ID:            currentAPISprint.ID,
		Name:          currentAPISprint.Name,
		IterationPath: currentAPISprint.Path,
	}
	if currentAPISprint.Attributes.StartDate != nil {
		sprintOutput.StartDate = currentAPISprint.Attributes.StartDate.Format("2006-01-02")
	}
	if currentAPISprint.Attributes.FinishDate != nil {
		sprintOutput.EndDate = currentAPISprint.Attributes.FinishDate.Format("2006-01-02")
	}

	format, _ := request.Params.Arguments["format"].(string)
	if strings.ToLower(format) == "json" {
		jsonStr, err := formatSprintOutputToJSON(sprintOutput)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}

	return mcp.NewToolResultText(formatSprintOutputToText(sprintOutput)), nil
}

func (tool *SprintTool) handleListSprints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeCompletedStr, _ := request.Params.Arguments["include_completed"].(string)
	includeCompleted := strings.ToLower(includeCompletedStr) == "true"

	timeframe := "current,future"
	if includeCompleted {
		timeframe = "" // Empty timeframe often means all for this API, or rely on API default
	}

	apiResponse, err := tool.callIterationsAPI(timeframe)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var sprintOutputs []SprintOutput
	for _, apiSprint := range apiResponse.Value {
		sprint := SprintOutput{
			ID:            apiSprint.ID,
			Name:          apiSprint.Name,
			IterationPath: apiSprint.Path,
		}
		if apiSprint.Attributes.StartDate != nil {
			sprint.StartDate = apiSprint.Attributes.StartDate.Format("2006-01-02")
		}
		if apiSprint.Attributes.FinishDate != nil {
			sprint.EndDate = apiSprint.Attributes.FinishDate.Format("2006-01-02")
		}
		sprintOutputs = append(sprintOutputs, sprint)
	}

	format, _ := request.Params.Arguments["format"].(string)
	if strings.ToLower(format) == "json" {
		jsonStr, err := formatSprintsOutputToJSON(sprintOutputs)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	}

	return mcp.NewToolResultText(formatSprintsOutputToText(sprintOutputs)), nil
}
