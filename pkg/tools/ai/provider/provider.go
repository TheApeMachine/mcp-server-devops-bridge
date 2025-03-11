// Package provider defines interfaces and implementations for various AI providers
package provider

import (
	"github.com/openai/openai-go"
)

// Message represents a generic message in a conversation
type Message struct {
	Role    string // "user", "assistant", "system", or "tool"
	Content string // The text content of the message
}

// ToolCall represents a function call from an AI model
type ToolCall struct {
	ID        string            // The ID of the tool call
	Name      string            // The name of the function being called
	Arguments map[string]string // The arguments for the function
}

// LLMProvider defines a common interface for all language model providers
type LLMProvider interface {
	// Generate produces text based on the current context
	Generate() (string, error)

	// AddUserMessage adds a user message to the conversation
	AddUserMessage(content string) error

	// AddSystemMessage adds a system message to the conversation
	AddSystemMessage(content string) error

	// AddToolMessage adds a tool response message to the conversation
	AddToolMessage(toolCallID string, content string) error

	// GetModel returns the current model being used
	GetModel() string

	// SetModel sets the model to use
	SetModel(model string) error
}

// ToolCallProvider extends LLMProvider with tool calling capabilities
type ToolCallProvider interface {
	LLMProvider

	// AddTools adds function calling tools to the provider
	AddTools(tools []openai.ChatCompletionToolParam) error

	// GetToolCalls returns any tool calls from the last generation
	GetToolCalls() []openai.ChatCompletionMessageToolCall
}

// MemoryItem represents text information to be stored
type MemoryItem struct {
	Text     string            // The text content to remember
	Metadata map[string]string // Additional information about the text
}

// MemorySearchQuery contains options for searching stored memories
type MemorySearchQuery struct {
	Text     string   // Natural language search query for semantic similarity
	Keywords []string // Specific keywords to look for in memories
	Limit    int      // Maximum number of results to return
}

// MemoryProvider extends LLMProvider with memory capabilities
type MemoryProvider interface {
	LLMProvider

	// StoreMemory stores information in the provider's memory
	StoreMemory(memory MemoryItem) error

	// QueryMemory retrieves information from the provider's memory
	QueryMemory(query MemorySearchQuery) ([]MemoryItem, error)
}
