package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/slack-go/slack"
)

// Global clients
var (
	githubClient *github.Client
	slackClient  *slack.Client
	n8nBaseURL   string
	n8nAPIKey    string
)

// Configuration for integrations
type IntegrationsConfig struct {
	GithubToken string
	SlackToken  string
	N8NBaseURL  string // Base URL of your N8N instance
	N8NAPIKey   string // API key for authentication
	OpenAIToken string
	QdrantURL   string
	QdrantAPIKey string
	Neo4jURL     string
	Neo4jUser    string
	Neo4jPassword string
}

// Initialize integration clients
func initializeIntegrationClients(config IntegrationsConfig) error {
	// Initialize GitHub client
	if config.GithubToken != "" {
		ts := github.BasicAuthTransport{
			Username: strings.TrimSpace(config.GithubToken),
		}
		githubClient = github.NewClient(ts.Client())
	}

	// Initialize Slack client
	if config.SlackToken != "" {
		slackClient = slack.New(config.SlackToken)
	}

	// Initialize N8N configuration
	if config.N8NBaseURL != "" && config.N8NAPIKey != "" {
		n8nBaseURL = strings.TrimRight(config.N8NBaseURL, "/")
		n8nAPIKey = config.N8NAPIKey
	}

	return nil
}

func addIntegrationTools(s *server.MCPServer) {
	// GitHub Tools
	addGitHubTools(s)

	// Slack Tools
	addSlackTools(s)

	// N8N Tools
	addN8NTools(s)

	// Cross-cutting Tools
	addCrossCuttingTools(s)
}

func addCrossCuttingTools(s *server.MCPServer) {
	// Daily Standup Report
	standupReportTool := mcp.NewTool("generate_standup_report",
		mcp.WithDescription("Generate a daily standup report for multiple teams"),
		mcp.WithString("teams",
			mcp.Required(),
			mcp.Description("Comma-separated list of team names"),
		),
		mcp.WithString("date",
			mcp.Description("Date for the report (YYYY-MM-DD format). Defaults to today"),
		),
	)
	s.AddTool(standupReportTool, handleGenerateStandupReport)

	// Generate Status Report
	statusReportTool := mcp.NewTool("generate_status_report",
		mcp.WithDescription("Generate a comprehensive status report"),
		mcp.WithString("sprint_id",
			mcp.Description("Sprint ID to report on"),
		),
		mcp.WithString("work_items",
			mcp.Description("Comma-separated list of work item IDs"),
		),
		mcp.WithBoolean("include_prs",
			mcp.Description("Include related pull requests"),
		),
		mcp.WithBoolean("include_discussions",
			mcp.Description("Include related discussions"),
		),
	)
	s.AddTool(statusReportTool, handleGenerateStatusReport)

	// Send Work Item Reminder
	reminderTool := mcp.NewTool("send_work_item_reminder",
		mcp.WithDescription("Send Slack reminders about stale work items"),
		mcp.WithNumber("work_item_id",
			mcp.Required(),
			mcp.Description("Work item ID"),
		),
		mcp.WithString("slack_user",
			mcp.Required(),
			mcp.Description("Slack user ID to notify"),
		),
		mcp.WithString("message",
			mcp.Description("Custom reminder message"),
		),
	)
	s.AddTool(reminderTool, handleSendWorkItemReminder)
}

// Cross-cutting Handlers
func handleGenerateStatusReport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sprintID, hasSprint := request.Params.Arguments["sprint_id"].(string)
	workItemsStr, hasWorkItems := request.Params.Arguments["work_items"].(string)
	includePRs, _ := request.Params.Arguments["include_prs"].(bool)
	includeDiscussions, _ := request.Params.Arguments["include_discussions"].(bool)

	// Initialize blocks for Slack message
	var blocks []slack.Block
	blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Status Report", false, false)))

	// Get sprint details if specified
	if hasSprint {
		// Get sprint details using the work item tracking client
		// Note: We'll use a simpler approach since we don't have direct iteration access
		blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Sprint Information", false, false)))

		sprintSection := slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Sprint ID:* %s", sprintID), false, false),
			nil, nil,
		)
		blocks = append(blocks, sprintSection, slack.NewDividerBlock())
	}

	// Get work item details
	if hasWorkItems {
		blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Work Items", false, false)))

		workItemIDs := strings.Split(workItemsStr, ",")
		for _, idStr := range workItemIDs {
			id, err := strconv.Atoi(idStr)
			if err != nil {
				continue
			}

			workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
				Id:      &id,
				Project: &config.Project,
			})
			if err != nil {
				continue
			}

			fields := *workItem.Fields
			title := fields["System.Title"].(string)
			state := fields["System.State"].(string)
			assignedTo := "Unassigned"
			if assigned, ok := fields["System.AssignedTo"].(map[string]interface{}); ok {
				assignedTo = assigned["displayName"].(string)
			}

			itemSection := slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*#%d:* %s\n*State:* %s\n*Assigned to:* %s",
					id, title, state, assignedTo), false, false),
				nil, nil,
			)
			blocks = append(blocks, itemSection)
		}
		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Get related PRs if requested
	if includePRs {
		blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Related Pull Requests", false, false)))

		// Get PRs linked to work items
		for _, idStr := range strings.Split(workItemsStr, ",") {
			id, err := strconv.Atoi(idStr)
			if err != nil {
				continue
			}

			// Get work item relations
			workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
				Id:      &id,
				Project: &config.Project,
				Expand:  &workitemtracking.WorkItemExpandValues.Relations,
			})
			if err != nil {
				continue
			}

			if workItem.Relations != nil {
				for _, relation := range *workItem.Relations {
					if strings.Contains(*relation.Url, "pullrequest") {
						prSection := slack.NewSectionBlock(
							slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*PR linked to #%d*\n<%s|View Pull Request>",
								id, *relation.Url), false, false),
							nil, nil,
						)
						blocks = append(blocks, prSection)
					}
				}
			}
		}
		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Get related discussions if requested
	if includeDiscussions {
		blocks = append(blocks, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "Related Discussions", false, false)))

		// Search for messages mentioning the work items
		for _, idStr := range strings.Split(workItemsStr, ",") {
			searchQuery := fmt.Sprintf("AB#%s", idStr)
			params := &slack.SearchParameters{
				Sort:  "timestamp",
				Count: 5,
			}
			results, err := slackClient.SearchMessages(searchQuery, *params)
			if err != nil {
				continue
			}

			for _, match := range results.Matches {
				messageSection := slack.NewSectionBlock(
					slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Channel:* #%s\n*From:* %s\n>%s\n<%s|View Message>",
						match.Channel.Name,
						match.Username,
						match.Text,
						match.Permalink), false, false),
					nil, nil,
				)
				blocks = append(blocks, messageSection)
			}
		}
	}

	// Convert blocks to JSON
	blocksJSON, err := json.Marshal(blocks)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal blocks: %v", err)), nil
	}

	return mcp.NewToolResultText(string(blocksJSON)), nil
}

func handleSendWorkItemReminder(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workItemID := int(request.Params.Arguments["work_item_id"].(float64))
	slackUser := request.Params.Arguments["slack_user"].(string)
	customMessage, hasCustomMessage := request.Params.Arguments["message"].(string)

	// Get work item details
	workItem, err := workItemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:      &workItemID,
		Project: &config.Project,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get work item: %v", err)), nil
	}

	fields := *workItem.Fields
	title := fields["System.Title"].(string)
	state := fields["System.State"].(string)

	// Build reminder message
	var message string
	if hasCustomMessage {
		message = customMessage
	} else {
		message = fmt.Sprintf("Reminder: Work item #%d (%s) is still in state '%s' and may need attention.",
			workItemID, title, state)
	}

	// Send message to user on Slack
	_, timestamp, err := slackClient.PostMessage(slackUser,
		slack.MsgOptionText(message, false),
		slack.MsgOptionAsUser(true))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send reminder: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Sent reminder about work item #%d to user %s (ts: %s)",
		workItemID, slackUser, timestamp)), nil
}

func handleGenerateStandupReport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	teamsStr := request.Params.Arguments["teams"].(string)
	teams := strings.Split(teamsStr, ",")

	dateStr, hasDate := request.Params.Arguments["date"].(string)
	var queryDate string
	if hasDate {
		queryDate = dateStr
	} else {
		queryDate = time.Now().Format("2006-01-02")
	}

	// Initialize blocks for Slack message
	var blocks []slack.Block
	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text",
			fmt.Sprintf("Daily Standup Report - %s", queryDate),
			false, false)))

	for _, team := range teams {
		team = strings.TrimSpace(team)
		blocks = append(blocks, slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text",
				fmt.Sprintf("Team: %s", team),
				false, false)))

		// Build WIQL query for work items updated today for this team
		wiql := fmt.Sprintf(`
			SELECT [System.Id], [System.Title], [System.State], [System.ChangedDate], [System.AssignedTo]
			FROM WorkItems
			WHERE 
				[System.TeamProject] = '%s'
				AND [System.ChangedDate] >= '%sT00:00:00Z'
				AND [System.ChangedDate] <= '%sT23:59:59Z'
				AND [System.TeamField] = '%s'
			ORDER BY [System.ChangedDate] DESC`,
			config.Project, queryDate, queryDate, team)

		// Execute the query
		wiqlPtr := &wiql
		workItemQueryArgs := workitemtracking.QueryByWiqlArgs{
			Wiql: &workitemtracking.Wiql{Query: wiqlPtr},
		}

		queryResults, err := workItemClient.QueryByWiql(ctx, workItemQueryArgs)
		if err != nil {
			continue
		}

		if queryResults.WorkItems == nil || len(*queryResults.WorkItems) == 0 {
			blocks = append(blocks, slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn",
					"No updates for this team today",
					false, false),
				nil, nil))
			continue
		}

		// Get full work item details
		var workItemIDs []int
		for _, item := range *queryResults.WorkItems {
			if item.Id != nil {
				workItemIDs = append(workItemIDs, *item.Id)
			}
		}

		workItems, err := workItemClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
			Ids:     &workItemIDs,
			Project: &config.Project,
		})
		if err != nil {
			continue
		}

		// Group work items by state
		stateGroups := make(map[string][]workitemtracking.WorkItem)
		for _, item := range *workItems {
			state := (*item.Fields)["System.State"].(string)
			stateGroups[state] = append(stateGroups[state], item)
		}

		// Add sections for each state
		for state, items := range stateGroups {
			var itemTexts []string
			for _, item := range items {
				fields := *item.Fields
				title := fields["System.Title"].(string)
				assignedTo := "Unassigned"
				if assigned, ok := fields["System.AssignedTo"].(map[string]interface{}); ok {
					assignedTo = assigned["displayName"].(string)
				}

				itemTexts = append(itemTexts, fmt.Sprintf("• #%d: %s (Assigned: %s)",
					*item.Id, title, assignedTo))
			}

			blocks = append(blocks,
				slack.NewSectionBlock(
					slack.NewTextBlockObject("mrkdwn",
						fmt.Sprintf("*%s*\n%s",
							state,
							strings.Join(itemTexts, "\n")),
						false, false),
					nil, nil))
		}

		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Convert blocks to JSON
	blocksJSON, err := json.Marshal(blocks)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal blocks: %v", err)), nil
	}

	return mcp.NewToolResultText(string(blocksJSON)), nil
}
