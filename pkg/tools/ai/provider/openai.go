package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
)

type Memory struct {
	Documents []Document `json:"documents" jsonschema_description:"Unstructured text you want to remember"`
	Cypher    string     `json:"cypher" jsonschema_description:"Cypher query to created a memory graph"`
}

type Document struct {
	Text     string     `json:"text" jsonschema_description:"The text you want to remember"`
	Metadata []Metadata `json:"metadata" jsonschema_description:"Metadata about the document"`
}

type Metadata struct {
	Key   string `json:"key" jsonschema_description:"Metadata key"`
	Value string `json:"value" jsonschema_description:"Metadata value"`
}

type MemoryQuery struct {
	Questions []string `json:"questions" jsonschema_description:"Natural language search query for semantic similarity"`
	Keywords  []string `json:"keywords" jsonschema_description:"Specific keywords to look for in memories"`
	Cypher    string   `json:"cypher" jsonschema_description:"Cypher query to find related memories through relationships"`
}

func GenerateSchema[T any]() any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

// OpenAIProvider implements LLMProvider for OpenAI models
type OpenAIProvider struct {
	client        *openai.Client
	model         string
	params        openai.ChatCompletionNewParams
	tools         []openai.ChatCompletionToolParam
	lastToolCalls []openai.ChatCompletionMessageToolCall
}

// NewOpenAIProvider creates a new provider for OpenAI
func NewOpenAIProvider() *OpenAIProvider {
	provider := &OpenAIProvider{
		client: openai.NewClient(),
		model:  openai.ChatModelGPT4oMini,
		params: openai.ChatCompletionNewParams{
			Messages: openai.F([]openai.ChatCompletionMessageParamUnion{}),
			Model:    openai.F(openai.ChatModelGPT4oMini),
		},
	}

	return provider
}

// Generate produces a response from OpenAI
func (provider *OpenAIProvider) Generate() (string, error) {
	if len(provider.tools) > 0 {
		provider.params.Tools = openai.F(provider.tools)
	}

	provider.params.Model = openai.F(provider.model)

	ctx := context.Background()
	chat, err := provider.client.Chat.Completions.New(ctx, provider.params)
	if err != nil {
		return "", fmt.Errorf("openai completion error: %w", err)
	}

	// Store the message in the conversation history
	provider.params.Messages.Value = append(provider.params.Messages.Value, chat.Choices[0].Message)

	// Save tool calls for later retrieval
	provider.lastToolCalls = chat.Choices[0].Message.ToolCalls

	content := chat.Choices[0].Message.Content
	if content == "" && len(provider.lastToolCalls) == 0 {
		return "", errors.New("no content or tool calls in response")
	}

	return content, nil
}

// AddUserMessage adds a user message to the conversation
func (provider *OpenAIProvider) AddUserMessage(content string) error {
	provider.params.Messages.Value = append(provider.params.Messages.Value,
		openai.UserMessage(content))
	return nil
}

// AddSystemMessage adds a system message to the conversation
func (provider *OpenAIProvider) AddSystemMessage(content string) error {
	// Find existing system message or add a new one
	for i, msg := range provider.params.Messages.Value {
		_, ok := msg.(*openai.ChatCompletionSystemMessageParam)
		if ok {
			// Replace existing system message
			provider.params.Messages.Value[i] = openai.SystemMessage(content)
			return nil
		}
	}

	// No existing system message, add it at the beginning
	provider.params.Messages.Value = append(
		[]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(content)},
		provider.params.Messages.Value...,
	)
	return nil
}

// AddToolMessage adds a tool response message to the conversation
func (provider *OpenAIProvider) AddToolMessage(toolCallID string, content string) error {
	provider.params.Messages.Value = append(provider.params.Messages.Value,
		openai.ToolMessage(toolCallID, content))
	return nil
}

// GetModel returns the current model being used
func (provider *OpenAIProvider) GetModel() string {
	return provider.model
}

// SetModel sets the model to use
func (provider *OpenAIProvider) SetModel(model string) error {
	supportedModels := map[string]bool{
		openai.ChatModelGPT4oMini:         true,
		openai.ChatModelGPT4o:             true,
		openai.ChatModelGPT4:              true,
		openai.ChatModelGPT4VisionPreview: true,
	}

	if !supportedModels[model] {
		return fmt.Errorf("unsupported model: %s", model)
	}

	provider.model = model
	return nil
}

// AddTools adds function calling tools to the provider
func (provider *OpenAIProvider) AddTools(tools []openai.ChatCompletionToolParam) error {
	provider.tools = tools
	return nil
}

// GetToolCalls returns any tool calls from the last generation
func (provider *OpenAIProvider) GetToolCalls() []openai.ChatCompletionMessageToolCall {
	return provider.lastToolCalls
}
