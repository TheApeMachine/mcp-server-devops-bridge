package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

type GitHubTool struct {
	handle mcp.Tool
	client *github.Client
}

func NewGitHubTool() core.Tool {
	return &GitHubTool{
		client: github.NewClient(nil),
		handle: mcp.NewTool(
			"github",
			mcp.WithDescription("Interact with GitHub"),
			mcp.WithString(
				"operation",
				mcp.Required(),
				mcp.Description("The operation to perform (list_pull_requests, get_pull_request, review_pull_request, comment_on_code, search_code)"),
			),
			mcp.WithString(
				"repo",
				mcp.Description("The repository to interact with"),
			),
			mcp.WithString(
				"pr_number",
				mcp.Description("The pull request number to interact with"),
			),
			mcp.WithString(
				"body",
				mcp.Description("The body of the review comment"),
			),
			mcp.WithString(
				"event_type",
				mcp.Description("The event type of the review comment"),
			),
			mcp.WithString(
				"commit_id",
				mcp.Description("The commit ID to comment on"),
			),
			mcp.WithString(
				"path",
				mcp.Description("The path to comment on"),
			),
			mcp.WithNumber(
				"line",
				mcp.Description("The line number to comment on"),
			),
			mcp.WithString(
				"query",
				mcp.Description("The query to search code with"),
			),
			mcp.WithString(
				"repo",
				mcp.Description("The repository to search code in"),
			),
		),
	}
}

func (tool *GitHubTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *GitHubTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch request.Params.Arguments["operation"].(string) {
	case "list_pull_requests":
		return tool.handleListPullRequests(ctx, request)
	case "get_pull_request":
		return tool.handleGetPullRequest(ctx, request)
	case "review_pull_request":
		return tool.handleReviewPullRequest(ctx, request)
	case "comment_on_code":
		return tool.handleCommentOnCode(ctx, request)
	case "search_code":
		return tool.handleSearchCode(ctx, request)
	}

	return nil, nil
}

// GitHub Handlers
func (tool *GitHubTool) handleListPullRequests(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	prs, _, err := tool.client.PullRequests.List(ctx, parts[0], parts[1], opts)
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

func (tool *GitHubTool) handleGetPullRequest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo := request.Params.Arguments["repo"].(string)
	prNumber := int(request.Params.Arguments["pr_number"].(float64))

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return mcp.NewToolResultError("Invalid repository format. Use owner/repo"), nil
	}

	pr, _, err := tool.client.PullRequests.Get(ctx, parts[0], parts[1], prNumber)
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

func (tool *GitHubTool) handleReviewPullRequest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	_, _, err := tool.client.PullRequests.CreateReview(ctx, parts[0], parts[1], prNumber, review)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create review: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully submitted review on PR #%d", prNumber)), nil
}

func (tool *GitHubTool) handleCommentOnCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	_, _, err := tool.client.PullRequests.CreateComment(ctx, parts[0], parts[1], prNumber, comment)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create comment: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully added comment on line %d of %s", line, path)), nil
}

func (tool *GitHubTool) handleSearchCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	results, _, err := tool.client.Search.Code(ctx, searchQuery, opts)
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
