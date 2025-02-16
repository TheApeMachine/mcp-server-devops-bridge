package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func addGitHubTools(s *server.MCPServer) {
	// List Pull Requests
	listPRsTool := mcp.NewTool("list_pull_requests",
		mcp.WithDescription("List open pull requests for a repository"),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name (owner/repo)"),
		),
		mcp.WithString("state",
			mcp.Description("State of PRs to list (open, closed, all)"),
			mcp.Enum("open", "closed", "all"),
		),
		mcp.WithString("sort",
			mcp.Description("Sort field (created, updated, popularity, long-running)"),
			mcp.Enum("created", "updated", "popularity", "long-running"),
		),
	)
	s.AddTool(listPRsTool, handleListPullRequests)

	// Get Pull Request Details
	getPRTool := mcp.NewTool("get_pull_request",
		mcp.WithDescription("Get detailed information about a specific PR"),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name (owner/repo)"),
		),
		mcp.WithNumber("pr_number",
			mcp.Required(),
			mcp.Description("Pull request number"),
		),
	)
	s.AddTool(getPRTool, handleGetPullRequest)

	// Review Pull Request
	reviewPRTool := mcp.NewTool("review_pull_request",
		mcp.WithDescription("Submit a review on a pull request"),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name (owner/repo)"),
		),
		mcp.WithNumber("pr_number",
			mcp.Required(),
			mcp.Description("Pull request number"),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.Description("Review comment body"),
		),
		mcp.WithString("event_type",
			mcp.Required(),
			mcp.Description("Review event type"),
			mcp.Enum("APPROVE", "REQUEST_CHANGES", "COMMENT"),
		),
	)
	s.AddTool(reviewPRTool, handleReviewPullRequest)

	// Comment on Code
	commentOnCodeTool := mcp.NewTool("comment_on_code",
		mcp.WithDescription("Leave a review comment on specific code lines"),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name (owner/repo)"),
		),
		mcp.WithNumber("pr_number",
			mcp.Required(),
			mcp.Description("Pull request number"),
		),
		mcp.WithString("commit_id",
			mcp.Required(),
			mcp.Description("Commit SHA"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("File path"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("Line number"),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.Description("Comment body"),
		),
	)
	s.AddTool(commentOnCodeTool, handleCommentOnCode)

	// Search Code
	searchCodeTool := mcp.NewTool("search_code",
		mcp.WithDescription("Search through repository code"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query"),
		),
		mcp.WithString("repo",
			mcp.Description("Repository to search in (owner/repo)"),
		),
		mcp.WithString("path",
			mcp.Description("Path to search in"),
		),
		mcp.WithString("language",
			mcp.Description("Language to filter by"),
		),
	)
	s.AddTool(searchCodeTool, handleSearchCode)
}

// GitHub Handlers
func handleListPullRequests(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo := request.Params.Arguments["repo"].(string)
	state, hasState := request.Params.Arguments["state"].(string)
	sort, hasSort := request.Params.Arguments["sort"].(string)

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return mcp.NewToolResultError("Invalid repository format. Use owner/repo"), nil
	}

	opts := &github.PullRequestListOptions{
		State: "open",
	}
	if hasState {
		opts.State = state
	}
	if hasSort {
		opts.Sort = sort
	}

	prs, _, err := githubClient.PullRequests.List(ctx, parts[0], parts[1], opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list pull requests: %v", err)), nil
	}

	var results []string
	for _, pr := range prs {
		results = append(results, fmt.Sprintf("#%d: %s\nState: %s\nCreated: %s\nURL: %s\n---",
			pr.GetNumber(),
			pr.GetTitle(),
			pr.GetState(),
			pr.GetCreatedAt().Format("2006-01-02 15:04:05"),
			pr.GetHTMLURL()))
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No pull requests found"), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

func handleGetPullRequest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo := request.Params.Arguments["repo"].(string)
	prNumber := int(request.Params.Arguments["pr_number"].(float64))

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return mcp.NewToolResultError("Invalid repository format. Use owner/repo"), nil
	}

	pr, _, err := githubClient.PullRequests.Get(ctx, parts[0], parts[1], prNumber)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get pull request: %v", err)), nil
	}

	result := fmt.Sprintf("Title: %s\nState: %s\nCreated: %s\nAuthor: %s\nDescription:\n%s\n\nReviews Required: %d\nMergeable: %v\nURL: %s",
		pr.GetTitle(),
		pr.GetState(),
		pr.GetCreatedAt().Format("2006-01-02 15:04:05"),
		pr.GetUser().GetLogin(),
		pr.GetBody(),
		len(pr.RequestedReviewers),
		pr.GetMergeable(),
		pr.GetHTMLURL())

	return mcp.NewToolResultText(result), nil
}

func handleReviewPullRequest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo := request.Params.Arguments["repo"].(string)
	prNumber := int(request.Params.Arguments["pr_number"].(float64))
	body := request.Params.Arguments["body"].(string)
	eventType := request.Params.Arguments["event_type"].(string)

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return mcp.NewToolResultError("Invalid repository format. Use owner/repo"), nil
	}

	review := &github.PullRequestReviewRequest{
		Body:  &body,
		Event: &eventType,
	}

	_, _, err := githubClient.PullRequests.CreateReview(ctx, parts[0], parts[1], prNumber, review)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create review: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully submitted review on PR #%d", prNumber)), nil
}

func handleCommentOnCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo := request.Params.Arguments["repo"].(string)
	prNumber := int(request.Params.Arguments["pr_number"].(float64))
	commitID := request.Params.Arguments["commit_id"].(string)
	path := request.Params.Arguments["path"].(string)
	line := int(request.Params.Arguments["line"].(float64))
	body := request.Params.Arguments["body"].(string)

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return mcp.NewToolResultError("Invalid repository format. Use owner/repo"), nil
	}

	comment := &github.PullRequestComment{
		Body:     &body,
		Path:     &path,
		Position: &line,
		CommitID: &commitID,
	}

	_, _, err := githubClient.PullRequests.CreateComment(ctx, parts[0], parts[1], prNumber, comment)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create comment: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully added comment on line %d of %s", line, path)), nil
}

func handleSearchCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.Params.Arguments["query"].(string)
	repo, hasRepo := request.Params.Arguments["repo"].(string)
	path, hasPath := request.Params.Arguments["path"].(string)
	language, hasLanguage := request.Params.Arguments["language"].(string)

	// Build search query
	searchQuery := query
	if hasRepo {
		searchQuery += fmt.Sprintf(" repo:%s", repo)
	}
	if hasPath {
		searchQuery += fmt.Sprintf(" path:%s", path)
	}
	if hasLanguage {
		searchQuery += fmt.Sprintf(" language:%s", language)
	}

	opts := &github.SearchOptions{
		TextMatch: true,
	}

	results, _, err := githubClient.Search.Code(ctx, searchQuery, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search code: %v", err)), nil
	}

	var output []string
	for _, item := range results.CodeResults {
		output = append(output, fmt.Sprintf("File: %s\nRepo: %s\nURL: %s\n---",
			item.GetPath(),
			item.GetRepository().GetFullName(),
			item.GetHTMLURL()))
	}

	if len(output) == 0 {
		return mcp.NewToolResultText("No results found"), nil
	}

	return mcp.NewToolResultText(strings.Join(output, "\n")), nil
}
