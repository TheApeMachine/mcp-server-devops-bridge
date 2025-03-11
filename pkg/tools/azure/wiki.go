package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/wiki"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

type WikiTool struct {
	client wiki.Client
	config AzureDevOpsConfig
	handle mcp.Tool
}

func NewWikiTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	wc, err := wiki.NewClient(context.Background(), conn)

	if err != nil {
		return nil
	}

	return &WikiTool{
		client: wc,
		config: config,
		handle: mcp.NewTool(
			"wiki",
		),
	}
}

func (tool *WikiTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *WikiTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch request.Params.Arguments["operation"].(string) {
	case "manage_wiki_page":
		return tool.handleManageWikiPage(ctx, request)
	case "get_wiki_page":
		return tool.handleGetWikiPage(ctx, request)
	case "list_wiki_pages":
		return tool.handleListWikiPages(ctx, request)
	case "search_wiki":
		return tool.handleSearchWiki(ctx, request)
	}

	return nil, nil
}

func (tool *WikiTool) handleManageWikiPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := request.Params.Arguments["path"].(string)
	content := request.Params.Arguments["content"].(string)
	// Note: Comments are not supported by the Azure DevOps Wiki API
	_, _ = request.Params.Arguments["comment"].(string)

	// Get wiki identifier
	wikiIdentifier := fmt.Sprintf("%s.wiki", tool.config.Project)

	_, err := tool.client.CreateOrUpdatePage(ctx, wiki.CreateOrUpdatePageArgs{
		WikiIdentifier: &wikiIdentifier,
		Path:           &path,
		Project:        &tool.config.Project,
		Parameters: &wiki.WikiPageCreateOrUpdateParameters{
			Content: &content,
		},
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to manage wiki page: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully managed wiki page: %s", path)), nil
}

func (tool *WikiTool) handleGetWikiPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := request.Params.Arguments["path"].(string)
	includeChildren, _ := request.Params.Arguments["include_children"].(bool)

	recursionLevel := "none"
	if includeChildren {
		recursionLevel = "oneLevel"
	}

	// Build the URL with query parameters
	wikiIdentifier := fmt.Sprintf("%s.wiki", tool.config.Project)
	baseURL := fmt.Sprintf("%s/%s/_apis/wiki/wikis/%s/pages",
		tool.config.OrganizationURL,
		tool.config.Project,
		wikiIdentifier)

	queryParams := url.Values{}
	queryParams.Add("path", path)
	queryParams.Add("recursionLevel", recursionLevel)
	queryParams.Add("includeContent", "true")
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get wiki page: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get wiki page. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var wikiResponse struct {
		Content  string `json:"content"`
		SubPages []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"subPages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&wikiResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Format result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== %s ===\n\n", path))
	result.WriteString(wikiResponse.Content)

	if includeChildren && len(wikiResponse.SubPages) > 0 {
		result.WriteString("\n\nSub-pages:\n")
		for _, subPage := range wikiResponse.SubPages {
			result.WriteString(fmt.Sprintf("\n=== %s ===\n", subPage.Path))
			result.WriteString(subPage.Content)
			result.WriteString("\n")
		}
	}

	return mcp.NewToolResultText(result.String()), nil
}

func (tool *WikiTool) handleListWikiPages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := request.Params.Arguments["path"].(string)
	recursive, _ := request.Params.Arguments["recursive"].(bool)

	recursionLevel := "oneLevel"
	if recursive {
		recursionLevel = "full"
	}

	// Build the URL with query parameters
	wikiIdentifier := fmt.Sprintf("%s.wiki", tool.config.Project)
	baseURL := fmt.Sprintf("%s/%s/_apis/wiki/wikis/%s/pages",
		tool.config.OrganizationURL,
		tool.config.Project,
		wikiIdentifier)

	queryParams := url.Values{}
	if path != "" {
		queryParams.Add("path", path)
	}
	queryParams.Add("recursionLevel", recursionLevel)
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list wiki pages: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list wiki pages. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var listResponse struct {
		Value []struct {
			Path       string `json:"path"`
			RemotePath string `json:"remotePath"`
			IsFolder   bool   `json:"isFolder"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Format result
	var result strings.Builder
	var locationText string
	if path != "" {
		locationText = " in " + path
	}
	result.WriteString(fmt.Sprintf("Wiki pages%s:\n\n", locationText))

	for _, item := range listResponse.Value {
		prefix := "📄 "
		if item.IsFolder {
			prefix = "📁 "
		}
		result.WriteString(fmt.Sprintf("%s%s\n", prefix, item.Path))
	}

	return mcp.NewToolResultText(result.String()), nil
}

func (tool *WikiTool) handleSearchWiki(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.Params.Arguments["query"].(string)
	path, hasPath := request.Params.Arguments["path"].(string)

	// First, get all pages (potentially under the specified path)
	baseURL := fmt.Sprintf("%s/%s/_apis/wiki/wikis/%s.wiki/pages",
		tool.config.OrganizationURL,
		tool.config.Project,
		tool.config.Project)

	queryParams := url.Values{}
	if hasPath {
		queryParams.Add("path", path)
	}
	queryParams.Add("recursionLevel", "full")
	queryParams.Add("includeContent", "true")
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search wiki: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search wiki. Status: %d", resp.StatusCode)), nil
	}

	// Parse response
	var searchResponse struct {
		Value []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Search through the pages
	var results []string
	queryLower := strings.ToLower(query)
	for _, page := range searchResponse.Value {
		if strings.Contains(strings.ToLower(page.Content), queryLower) {
			// Extract a snippet of context around the match
			contentLower := strings.ToLower(page.Content)
			index := strings.Index(contentLower, queryLower)
			start := max(0, index-100)
			end := min(len(page.Content), index+len(query)+100)

			snippet := page.Content[start:end]
			if start > 0 {
				snippet = "..." + snippet
			}
			if end < len(page.Content) {
				snippet = snippet + "..."
			}

			results = append(results, fmt.Sprintf("Page: %s\nMatch: %s\n---\n", page.Path, snippet))
		}
	}

	if len(results) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No matches found for '%s'", query)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d matches:\n\n%s", len(results), strings.Join(results, "\n"))), nil
}
