package ai

import (
	"github.com/openai/openai-go"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// RegisterAITools registers all the AI tools with the main application
func RegisterAITools() []core.Tool {
	return []core.Tool{
		NewAgentTool(),
		NewListAgentsTool(),
		NewSendCommandTool(),
		NewSubscribeAgentTool(),
		NewKillAgentTool(),
		NewSendAgentMessageTool(),
	}
}

// GetAllToolsAsOpenAI returns all AI tools in OpenAI format
func GetAllToolsAsOpenAI() []openai.ChatCompletionToolParam {
	tools := RegisterAITools()
	openaiTools := make([]openai.ChatCompletionToolParam, 0, len(tools))

	for _, tool := range tools {
		if openaiTool, ok := tool.(interface {
			ToOpenAITool() openai.ChatCompletionToolParam
		}); ok {
			openaiTools = append(openaiTools, openaiTool.ToOpenAITool())
		}
	}

	return openaiTools
}
