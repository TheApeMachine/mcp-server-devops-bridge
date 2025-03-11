package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go"
)

// AnthropicProvider implements ToolCallProvider for Anthropic Claude models
type AnthropicProvider struct {
	client        *anthropic.Client
	model         string
	maxTokens     int64
	messages      []anthropic.MessageParam
	systemPrompt  string
	tools         []anthropic.ToolUnionUnionParam
	lastToolCalls []openai.ChatCompletionMessageToolCall
}

// NewAnthropicProvider creates a new provider for Anthropic Claude models
func NewAnthropicProvider() *AnthropicProvider {
	// NewClient() will use ANTHROPIC_API_KEY environment variable by default
	client := anthropic.NewClient()

	return &AnthropicProvider{
		client:    client,
		model:     "claude-3-5-sonnet-20240620", // Default model
		maxTokens: 4096,
		messages:  []anthropic.MessageParam{},
		tools:     []anthropic.ToolUnionUnionParam{},
	}
}

// Generate creates a completion using the Anthropic Claude API
func (p *AnthropicProvider) Generate() (string, error) {
	if len(p.messages) == 0 {
		return "", errors.New("no messages provided")
	}

	// Reset last tool calls
	p.lastToolCalls = nil

	params := anthropic.MessageNewParams{
		Model:     anthropic.F(p.model),
		MaxTokens: anthropic.Int(p.maxTokens),
		Messages:  anthropic.F(p.messages),
	}

	// Add system prompt if it exists
	if p.systemPrompt != "" {
		params.System = anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(p.systemPrompt),
		})
	}

	// Add tools if they exist
	if len(p.tools) > 0 {
		params.Tools = anthropic.F(p.tools)
	}

	response, err := p.client.Messages.New(context.Background(), params)
	if err != nil {
		return "", fmt.Errorf("failed to generate text: %w", err)
	}

	// Process the response content
	contentBlocks := []anthropic.ContentBlockParamUnion{}
	text := ""

	for _, block := range response.Content {
		switch block := block.AsUnion().(type) {
		case anthropic.TextBlock:
			text += block.Text
			contentBlocks = append(contentBlocks, anthropic.NewTextBlock(block.Text))
		case anthropic.ToolUseBlock:
			// Convert Anthropic tool calls to OpenAI format for compatibility
			toolCall := openai.ChatCompletionMessageToolCall{
				ID:   block.ID,
				Type: "function",
				Function: openai.ChatCompletionMessageToolCallFunction{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			}
			p.lastToolCalls = append(p.lastToolCalls, toolCall)

			// TODO: Add the tool use block to the message
			// This needs to be implemented using the correct Anthropic SDK method
			// We need to create a text representation for now
			textContent := fmt.Sprintf("[Tool Use: %s(%s)]", block.Name, string(block.Input))
			contentBlocks = append(contentBlocks, anthropic.NewTextBlock(textContent))
		}
	}

	// Add the assistant's response to the conversation history
	p.messages = append(p.messages, anthropic.NewAssistantMessage(contentBlocks...))

	return text, nil
}

// AddUserMessage adds a user message to the conversation
func (p *AnthropicProvider) AddUserMessage(content string) error {
	userMessage := anthropic.NewUserMessage(
		anthropic.NewTextBlock(content),
	)
	p.messages = append(p.messages, userMessage)
	return nil
}

// AddSystemMessage sets the system prompt for the conversation
func (p *AnthropicProvider) AddSystemMessage(content string) error {
	p.systemPrompt = content
	return nil
}

// AddToolMessage adds a tool result message to the conversation
func (p *AnthropicProvider) AddToolMessage(toolCallID string, content string) error {
	// Create a tool result block
	toolResult := anthropic.NewToolResultBlock(toolCallID, content, false)

	// Add the tool result as a user message
	p.messages = append(p.messages, anthropic.NewUserMessage(toolResult))
	return nil
}

// GetModel returns the current model being used
func (p *AnthropicProvider) GetModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *AnthropicProvider) SetModel(model string) error {
	p.model = anthropic.ModelClaude3_7SonnetLatest
	return nil
}

// AddTools adds function calling tools to the provider
func (p *AnthropicProvider) AddTools(openaiTools []openai.ChatCompletionToolParam) error {
	// Convert OpenAI tools to Anthropic tools
	for _, openaiTool := range openaiTools {
		if openaiTool.Type.Value != openai.ChatCompletionToolTypeFunction {
			continue // Skip non-function tools
		}

		// Extract function definition
		funcDef := openaiTool.Function.Value

		// Create Anthropic tool
		anthropicTool := anthropic.ToolParam{
			Name: anthropic.F(funcDef.Name.Value),
		}

		// Add description if available
		if funcDef.Description.Value != "" {
			anthropicTool.Description = anthropic.F(funcDef.Description.Value)
		}

		// Convert parameters
		var inputSchema map[string]interface{}
		paramBytes, err := json.Marshal(funcDef.Parameters.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal parameters: %w", err)
		}

		if err := json.Unmarshal(paramBytes, &inputSchema); err != nil {
			return fmt.Errorf("failed to unmarshal parameters: %w", err)
		}

		anthropicTool.InputSchema = anthropic.F[interface{}](inputSchema)
		p.tools = append(p.tools, anthropicTool)
	}

	return nil
}

// GetToolCalls returns any tool calls from the last generation
func (p *AnthropicProvider) GetToolCalls() []openai.ChatCompletionMessageToolCall {
	return p.lastToolCalls
}
