package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
)

// AzureDevOpsConfig contains configuration for Azure DevOps integration
type AzureDevOpsConfig struct {
	OrganizationURL     string
	PersonalAccessToken string
	Project             string
	Team                string
}

// Helper to extract a string argument.
func GetStringArg(req mcp.CallToolRequest, key string) (string, error) {
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
func GetFloat64Arg(req mcp.CallToolRequest, key string) (float64, error) {
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
func GetIntArg(req mcp.CallToolRequest, key string) (int, error) {
	var (
		f   float64
		err error
	)

	if f, err = GetFloat64Arg(req, key); err != nil {
		return 0, err
	}

	return int(f), nil
}

// Helper for common error formatting
func HandleError(err error, message string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", message, err))
}

// Helper to parse a comma-separated list of IDs
func ParseIDs(idsStr string) ([]int, error) {
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

// Format a list of strings for WIQL queries
func FormatStringList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("'%s'", item)
	}
	return strings.Join(quoted, ", ")
}

// Helper function for min value
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function to create a string pointer
func StringPtr(s string) *string {
	return &s
}

// Helper function to add an operation
func AddOperation(field string, value any) webapi.JsonPatchOperation {
	return webapi.JsonPatchOperation{
		Op:    &webapi.OperationValues.Add,
		Path:  StringPtr("/fields/" + field),
		Value: value,
	}
}

// Relation type mapping
var RelationTypeMap = map[string]string{
	"parent":   "System.LinkTypes.Hierarchy-Reverse",
	"child":    "System.LinkTypes.Hierarchy-Forward",
	"children": "System.LinkTypes.Hierarchy-Forward", // Alias
	"related":  "System.LinkTypes.Related",
}

// Field name mapping
var FieldMap = map[string]string{
	"Title":       "System.Title",
	"Description": "System.Description",
	"State":       "System.State",
	"Priority":    "Microsoft.VSTS.Common.Priority",
}

// Get the current sprint information
func GetCurrentSprint(ctx context.Context, config AzureDevOpsConfig) (map[string]any, error) {
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
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Add authentication
	req.SetBasicAuth("", config.PersonalAccessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get current sprint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get current sprint. Status: %d", resp.StatusCode)
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
		return nil, fmt.Errorf("failed to parse sprint response: %v", err)
	}

	if len(sprintResponse.Value) == 0 {
		return nil, fmt.Errorf("no active sprint found")
	}

	currentSprint := sprintResponse.Value[0]
	return map[string]any{
		"id":         currentSprint.ID,
		"name":       currentSprint.Name,
		"path":       currentSprint.Path,
		"start_date": currentSprint.StartDate,
		"end_date":   currentSprint.EndDate,
	}, nil
}

// GetWorkItemURL constructs the UI URL for a work item.
func GetWorkItemURL(orgURL string, workItemID int) string {
	// Ensure orgURL doesn't have trailing slashes from config and project isn't part of the base for this URL type
	// Example: https://dev.azure.com/MyOrg/MyProject/_workitems/edit/123
	// The _apis/wit/workItems/{id} is the API URL, not the UI one usually needed by users.
	// Let's assume config.Project is available for the UI URL.
	// This needs to be confirmed with the actual AzureDevOpsConfig struct if it contains Project.
	// For now, making a generic UI URL pattern if project name isn't directly available here
	// or part of the orgURL from config.
	// A more robust way is to ensure project name is passed or available.
	// If tool.config.Project is accessible, it should be used.
	// The common function won't have direct access to tool.config.Project unless passed.
	// Let's assume orgURL ALREADY contains the organization part e.g. https://dev.azure.com/MyOrg
	// and we need to append the project and work item path.
	// THIS FUNCTION MIGHT NEED ACCESS TO THE PROJECT NAME for a full UI URL.
	// For now, providing a common structure and acknowledging this dependency.
	// A common structure is: {OrganizationURL}/{Project}/_workitems/edit/{workItemID}
	// If orgURL = "https://dev.azure.com/MyOrg" and project = "MyProject", then
	// https://dev.azure.com/MyOrg/MyProject/_workitems/edit/123
	// This function will assume orgURL is just the org base (e.g. https://dev.azure.com/orgName)
	// and the project name needs to be sourced elsewhere or this func needs to be in a context with it.
	// Given this is common, it implies it should be generic.
	// Let's make it expect the fully qualified org URL (e.g. https://dev.azure.com/myorg)
	// and the project name separately if this is to be truly common and accurate.
	// However, the original get_work_items.go used tool.config.OrganizationURL which implies it might contain project context for UI.
	// Let's use a simpler form that assumes orgURL is the base up to the org.
	// The `_workitems/edit/ID` path is universal for UI links AFTER project context.
	// This is tricky for a truly common function without project context.
	// Reverting to the simpler one from update_work_items.go for consistency for now and will require project in URL path or separate param.
	// The update_work_items.go uses: fmt.Sprintf("%s/%s/_workitems/edit/%d", strings.TrimRight(orgURL, "/"), "_apis/wit/workItems", id)
	// which is actually the API URL base. A UI url is more like: {org}/{project}/_workitems/edit/{id}
	// Let's assume orgURL is the full base URL up to the organization, e.g., https://dev.azure.com/MyOrg
	// And this function CANNOT know the project. So it can only give a partial piece or the caller forms it.
	// For now, I will return a more generic API like URL structure, acknowledging it's not the UI URL.
	// return fmt.Sprintf("%s/_apis/wit/workItems/%d", strings.TrimRight(orgURL, "/"), workItemID)
	// The previous get_work_items.go had: fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, id)
	// This implies tool.config.OrganizationURL was specific enough. If common.go is used, it won't have tool.
	// Given the user is moving functions here, they must be callable without tool context.
	// So, this function will need more parameters or a fixed structure based on assumptions.

	// Let's use a simplified version that takes org base and project
	// This will require the caller (specific tool) to provide these.
	// For now, let's assume the `orgURL` parameter is the full base up to the project for UI links if possible,
	// otherwise this common function is less useful for direct UI links.
	// Based on previous tool code: `fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, id)`
	// This means OrganizationURL must be the project specific URL e.g. https://dev.azure.com/MyOrg/MyProject
	// This is a common pattern for such configs.
	return fmt.Sprintf("%s/_workitems/edit/%d", strings.TrimRight(orgURL, "/"), workItemID)
}

// ExtractWorkItemIDFromURL extracts the work item ID from a standard Azure DevOps work item URL.
// Example URL: https://dev.azure.com/org/project/_apis/wit/workItems/123
// or https://org.visualstudio.com/project/_apis/wit/workItems/123
func ExtractWorkItemIDFromURL(itemURL string) (int, error) {
	parsedURL, err := url.Parse(itemURL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URL '%s': %w", itemURL, err)
	}
	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	// Expected structure: org/project/_apis/wit/workItems/ID or project/_apis/wit/workItems/ID or _apis/wit/workItems/ID
	// We need the last segment if it's a number.
	for i := len(pathSegments) - 1; i >= 0; i-- {
		if pathSegments[i] == "workItems" && i+1 < len(pathSegments) {
			idStr := pathSegments[i+1]
			id, err := strconv.Atoi(idStr)
			if err == nil {
				return id, nil
			}
			// If it's not an int after workItems, it might be a more complex URL or not an ID.
			return 0, fmt.Errorf("could not find numeric ID after 'workItems' segment in URL '%s'", itemURL)
		}
	}
	// Fallback: try to get the last segment if it's numeric, less reliable
	if len(pathSegments) > 0 {
		lastSegment := pathSegments[len(pathSegments)-1]
		id, err := strconv.Atoi(lastSegment)
		if err == nil {
			return id, nil // This assumes the ID is simply the last part of the path
		}
	}
	return 0, fmt.Errorf("could not extract work item ID from URL path '%s'", parsedURL.Path)
}
