package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/slack-go/slack"
)

func addSlackTools(s *server.MCPServer) {
	// Add Slack Blocks Format Prompt
	s.AddPrompt(mcp.NewPrompt("slack_blocks_format",
		mcp.WithPromptDescription("Helper for formatting Slack block messages"),
		mcp.WithArgument("message_type",
			mcp.ArgumentDescription("Type of message to format (status_report, notification, etc)"),
			mcp.RequiredArgument(),
		),
	), handleSlackBlocksFormatPrompt)

	// Search Messages
	searchMessagesTool := mcp.NewTool("search_messages",
		mcp.WithDescription("Search through Slack messages"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query"),
		),
		mcp.WithString("channel",
			mcp.Description("Channel to search in"),
		),
		mcp.WithString("time_range",
			mcp.Description("Time range (1d, 1w, 1m)"),
			mcp.Enum("1d", "1w", "1m"),
		),
	)
	s.AddTool(searchMessagesTool, handleSearchMessages)

	// Post Message
	postMessageTool := mcp.NewTool("post_message",
		mcp.WithDescription(`Post a new message to a Slack channel with optional Block Kit formatting.
Example blocks format:
[
    {
        "type": "header",
        "text": {
            "type": "plain_text",
            "text": "Your Header",
            "emoji": true
        }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "*Bold text* and _italic_ with <https://example.com|Click here>"
        }
    }
]

Supported block types:
- header (plain_text only)
- section (mrkdwn or plain_text)
- divider
- context (array of mrkdwn elements)`),
		mcp.WithString("channel",
			mcp.Required(),
			mcp.Description("Channel to post to (e.g., "+os.Getenv("DEAFULT_SLACK_CHANNEL")+", when in doubt, use this one)"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Fallback text message (displayed in notifications)"),
		),
		mcp.WithString("blocks",
			mcp.Description("Message blocks in JSON format following Slack's Block Kit structure"),
		),
		mcp.WithString("thread_ts",
			mcp.Description("Thread timestamp to reply to"),
		),
	)
	s.AddTool(postMessageTool, handlePostMessage)

	// Add Reaction
	addReactionTool := mcp.NewTool("add_reaction",
		mcp.WithDescription("Add an emoji reaction to a message"),
		mcp.WithString("channel",
			mcp.Required(),
			mcp.Description("Channel containing the message"),
		),
		mcp.WithString("timestamp",
			mcp.Required(),
			mcp.Description("Message timestamp"),
		),
		mcp.WithString("emoji",
			mcp.Required(),
			mcp.Description("Emoji name without colons"),
		),
	)
	s.AddTool(addReactionTool, handleAddReaction)
}

// Slack Handlers
func handleSearchMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	results, err := slackClient.SearchMessages(searchQuery, *params)
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

func handlePostMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	channel := request.Params.Arguments["channel"].(string)
	text := request.Params.Arguments["text"].(string)
	blocks, hasBlocks := request.Params.Arguments["blocks"].(string)
	threadTS, hasThread := request.Params.Arguments["thread_ts"].(string)

	opts := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	if hasBlocks {
		var blockSlice []map[string]interface{}
		if err := json.Unmarshal([]byte(blocks), &blockSlice); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid blocks JSON: %v", err)), nil
		}

		// Convert the generic blocks to Slack blocks
		var slackBlocks []slack.Block
		for _, block := range blockSlice {
			switch block["type"] {
			case "header":
				if text, ok := block["text"].(map[string]interface{}); ok {
					headerText := slack.NewTextBlockObject(text["type"].(string), text["text"].(string), false, false)
					slackBlocks = append(slackBlocks, slack.NewHeaderBlock(headerText))
				}
			case "section":
				if text, ok := block["text"].(map[string]interface{}); ok {
					sectionText := slack.NewTextBlockObject(text["type"].(string), text["text"].(string), false, false)
					slackBlocks = append(slackBlocks, slack.NewSectionBlock(sectionText, nil, nil))
				}
			case "divider":
				slackBlocks = append(slackBlocks, slack.NewDividerBlock())
			case "context":
				if elements, ok := block["elements"].([]interface{}); ok {
					var contextElements []slack.MixedElement
					for _, elem := range elements {
						if elemMap, ok := elem.(map[string]interface{}); ok {
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

	_, timestamp, err := slackClient.PostMessage(channel, opts...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to post message: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message posted successfully (ts: %s)", timestamp)), nil
}

func handleAddReaction(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	channel := request.Params.Arguments["channel"].(string)
	timestamp := request.Params.Arguments["timestamp"].(string)
	emoji := request.Params.Arguments["emoji"].(string)

	err := slackClient.AddReaction(emoji, slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add reaction: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Added :%s: reaction", emoji)), nil
}

func handleSlackBlocksFormatPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	messageType, exists := request.Params.Arguments["message_type"]
	if !exists {
		return nil, fmt.Errorf("message_type is required")
	}

	var template string
	switch messageType {
	case "status_report":
		template = `[
			{
				"type": "header",
				"text": {
					"type": "plain_text",
					"text": "Status Report",
					"emoji": true
				}
			},
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": "*Status:* Complete\n*Details:* Your status details here"
				}
			},
			{
				"type": "divider"
			},
			{
				"type": "context",
				"elements": [
					{
						"type": "mrkdwn",
						"text": "Generated on: " + time.Now().Format("2006-01-02 15:04:05")
					}
				]
			}
		]`
	case "notification":
		template = `[
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": "*New Notification*\nYour notification message here"
				}
			}
		]`
	}

	return mcp.NewGetPromptResult(
		"Slack Blocks Format Helper",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				"system",
				mcp.NewTextContent("You are a Slack message formatting expert. Help format messages using Slack's Block Kit."),
			),
			mcp.NewPromptMessage(
				"assistant",
				mcp.NewTextContent(fmt.Sprintf("Here's a template for a %s message using Slack blocks:\n\n```json\n%s\n```\n\nKey points:\n- Each block must have a 'type' field\n- Text objects must specify their type ('plain_text' or 'mrkdwn')\n- Context blocks require an array of elements\n- Headers only support plain_text", messageType, template)),
			),
		},
	), nil
}
