package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/slack-go/slack"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

type SlackTool struct {
	handle mcp.Tool
	client *slack.Client
}

func NewSlackTool() core.Tool {
	return &SlackTool{
		client: slack.New(
			os.Getenv("MARVIN_BOT_TOKEN"),
			slack.OptionAppLevelToken(os.Getenv("MARVIN_APP_TOKEN")),
		),
		handle: mcp.NewTool(
			"slack",
			mcp.WithDescription("Interact with Slack"),
			mcp.WithString(
				"operation",
				mcp.Required(),
				mcp.Description("The operation to perform (search_messages, post_message, add_reaction)"),
			),
			mcp.WithString(
				"channel",
				mcp.Description("The channel to post the message to"),
			),
			mcp.WithString(
				"text",
				mcp.Description("The text to post to the channel"),
			),
			mcp.WithString(
				"blocks",
				mcp.Description("The blocks to post to the channel"),
			),
			mcp.WithString(
				"thread_ts",
				mcp.Description("The timestamp of the thread to post to"),
			),
			mcp.WithString(
				"emoji",
				mcp.Description("The emoji to add to the message"),
			),
		),
	}
}

func (tool *SlackTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *SlackTool) Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (result *mcp.CallToolResult, err error) {
	switch request.Params.Arguments["operation"].(string) {
	case "search_messages":
		return tool.handleSearchMessages(request)
	case "post_message":
		return tool.handlePostMessage(request)
	case "add_reaction":
		return tool.handleAddReaction(request)
	}

	return nil, nil
}

// Slack Handlers
func (tool *SlackTool) handleSearchMessages(
	request mcp.CallToolRequest,
) (result *mcp.CallToolResult, err error) {
	query := request.Params.Arguments["query"].(string)
	channel, hasChannel := request.Params.Arguments["channel"].(string)
	timeRange, hasTimeRange := request.Params.Arguments["time_range"].(string)

	searchQuery := query
	if hasChannel {
		searchQuery = fmt.Sprintf("in:%s %s", channel, searchQuery)
	}
	if hasTimeRange {
		searchQuery = fmt.Sprintf("after:%s %s", timeRange, searchQuery)
	}

	params := &slack.SearchParameters{}
	results, err := tool.client.SearchMessages(searchQuery, *params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search messages: %v", err)), nil
	}

	var output []string
	for _, match := range results.Matches {
		output = append(output, fmt.Sprintf("Channel: %s\nUser: %s\nTime: %s\nMessage: %s\n---",
			match.Channel.Name,
			match.Username,
			match.Timestamp,
			match.Text))
	}

	if len(output) == 0 {
		return mcp.NewToolResultText("No messages found"), nil
	}

	return mcp.NewToolResultText(strings.Join(output, "\n")), nil
}

func (tool *SlackTool) handlePostMessage(
	request mcp.CallToolRequest,
) (result *mcp.CallToolResult, err error) {
	channel := request.Params.Arguments["channel"].(string)
	text := request.Params.Arguments["text"].(string)
	blocks, hasBlocks := request.Params.Arguments["blocks"].(string)
	threadTS, hasThread := request.Params.Arguments["thread_ts"].(string)

	opts := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	if hasBlocks {
		var blockSlice []map[string]any
		if err := json.Unmarshal([]byte(blocks), &blockSlice); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid blocks JSON: %v", err)), nil
		}

		// Convert the generic blocks to Slack blocks
		var slackBlocks []slack.Block
		for _, block := range blockSlice {
			switch block["type"] {
			case "header":
				if text, ok := block["text"].(map[string]any); ok {
					headerText := slack.NewTextBlockObject(text["type"].(string), text["text"].(string), false, false)
					slackBlocks = append(slackBlocks, slack.NewHeaderBlock(headerText))
				}
			case "section":
				if text, ok := block["text"].(map[string]any); ok {
					sectionText := slack.NewTextBlockObject(text["type"].(string), text["text"].(string), false, false)
					slackBlocks = append(slackBlocks, slack.NewSectionBlock(sectionText, nil, nil))
				}
			case "divider":
				slackBlocks = append(slackBlocks, slack.NewDividerBlock())
			case "context":
				if elements, ok := block["elements"].([]any); ok {
					var contextElements []slack.MixedElement
					for _, elem := range elements {
						if elemMap, ok := elem.(map[string]any); ok {
							if elemMap["type"] == "mrkdwn" {
								contextText := slack.NewTextBlockObject("mrkdwn", elemMap["text"].(string), false, false)
								contextElements = append(contextElements, contextText)
							}
						}
					}
					slackBlocks = append(slackBlocks, slack.NewContextBlock("", contextElements...))
				}
			}
		}

		opts = append(opts, slack.MsgOptionBlocks(slackBlocks...))
	}

	if hasThread {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	_, timestamp, err := tool.client.PostMessage(channel, opts...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to post message: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message posted successfully (ts: %s)", timestamp)), nil
}

func (tool *SlackTool) handleAddReaction(request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	channel := request.Params.Arguments["channel"].(string)
	timestamp := request.Params.Arguments["timestamp"].(string)
	emoji := request.Params.Arguments["emoji"].(string)

	err := tool.client.AddReaction(emoji, slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add reaction: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Added :%s: reaction", emoji)), nil
}
