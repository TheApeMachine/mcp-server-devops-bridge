package system

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
)

type Schema struct {
	Type     string `json:"type" jsonschema_description:"The type of command to run"`
	Function string `json:"function" jsonschema_description:"The function to run"`
}

// NewSchema creates a new Schema instance with the given allowed commands
func NewSchema(allowedCommands []string) *Schema {
	return &Schema{
		Type:     "function",
		Function: "system",
	}
}

// ToOpenAITool converts the Schema to an OpenAI tool format
func (s *Schema) ToOpenAITool(allowedCommands []string) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("system"),
			Description: openai.String("A linux bash command to run"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The command to run",
						"enum":        allowedCommands,
					},
				},
				"required": []string{"command"},
			}),
		}),
	}
}

type SystemTool struct {
	handle mcp.Tool
	cmds   []string
}

func NewSystemTool(cmds []string) *SystemTool {
	return &SystemTool{
		handle: mcp.NewTool(
			"system",
			mcp.WithDescription("Run a linux bash command"),
			mcp.WithString("command", mcp.Required(), mcp.Description("The command to run")),
		),
		cmds: cmds,
	}
}

// GetSchema returns the schema for this tool
func (tool *SystemTool) GetSchema() *Schema {
	return NewSchema(tool.cmds)
}

// ToOpenAITool converts the tool schema to OpenAI format
func (tool *SystemTool) ToOpenAITool() openai.ChatCompletionToolParam {
	schema := tool.GetSchema()
	return schema.ToOpenAITool(tool.cmds)
}

func (tool *SystemTool) Handle() mcp.Tool {
	return tool.handle
}
