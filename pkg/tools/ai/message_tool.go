package ai

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// SendAgentMessageTool enables sending messages to a topic for agent-to-agent communication
type SendAgentMessageTool struct {
	handle mcp.Tool
}

func NewSendAgentMessageTool() core.Tool {
	return &SendAgentMessageTool{
		handle: mcp.NewTool(
			"send_agent_message",
			mcp.WithDescription("Send a message to a topic for agent-to-agent communication"),
			mcp.WithString("topic", mcp.Required(), mcp.Description("Topic to send message to")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Message content")),
		),
	}
}

func (tool *SendAgentMessageTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *SendAgentMessageTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("send_agent_message"),
			Description: openai.String("Send a message to a topic for agent-to-agent communication"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"topic": map[string]any{
						"type":        "string",
						"description": "Topic to send message to",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Message content",
					},
				},
				"required": []string{"topic", "content"},
			}),
		}),
	}
}

func (tool *SendAgentMessageTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		topic   string
		content string
		ok      bool
	)

	if topic, ok = request.Params.Arguments["topic"].(string); !ok {
		return mcp.NewToolResultError("Topic is required"), nil
	}

	if content, ok = request.Params.Arguments["content"].(string); !ok {
		return mcp.NewToolResultError("Content is required"), nil
	}

	// Determine the sender ID (from context or use "system")
	senderID := "system"
	if contextID, ok := ctx.Value("agent_id").(string); ok && contextID != "" {
		senderID = contextID
	}

	// Create the message
	msg := Message{
		From:    senderID,
		Topic:   topic,
		Content: content,
	}

	// Publish the message
	err := messageBus.Publish(msg)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send message: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message sent to topic '%s'", topic)), nil
}
