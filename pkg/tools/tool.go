// Package tools provides interfaces and implementations for MCP tools
package tools

import (
	"context"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
)

// Standard errors for consistent error handling
var (
	ErrInvalidParams    = errors.New("invalid parameters")
	ErrNotImplemented   = errors.New("operation not implemented")
	ErrResourceNotFound = errors.New("resource not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrExternalAPIError = errors.New("external API error")
	ErrInternalError    = errors.New("internal server error")
)

// Tool defines the interface for all tools in the system
type Tool interface {
	// Handle returns the underlying MCP tool
	Handle() mcp.Tool

	// ToOpenAITool converts the tool to OpenAI format
	ToOpenAITool() openai.ChatCompletionToolParam

	// Handler processes tool requests and returns responses
	Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

	// Name returns the name of the tool
	Name() string
}

// BaseTool provides common functionality for all tools
type BaseTool struct {
	name   string
	handle mcp.Tool
}

// NewBaseTool creates a new BaseTool with the given name and handle
func NewBaseTool(name string, handle mcp.Tool) *BaseTool {
	return &BaseTool{
		name:   name,
		handle: handle,
	}
}

// Handle returns the MCP Tool definition
func (b *BaseTool) Handle() mcp.Tool {
	return b.handle
}

// Name returns the name of the tool
func (b *BaseTool) Name() string {
	return b.name
}

// WrapError wraps a domain error with a context message
func WrapError(err error, msg string) error {
	return errors.New(msg + ": " + err.Error())
}

// NewErrorResult creates a standard error result
func NewErrorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

// NewTextResult creates a standard text result
func NewTextResult(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

// GetOpenAITools converts a slice of tools to OpenAI format
func GetOpenAITools(tools []Tool) []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, len(tools))
	for i, tool := range tools {
		openaiTools[i] = tool.ToOpenAITool()
	}
	return openaiTools
}

// Handler processes tool requests and returns responses
type Handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// DefaultHandler provides a default implementation for tool handlers
func DefaultHandler(text string) Handler {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(text), nil
	}
}

// ErrorHandler provides an error implementation for tool handlers
func ErrorHandler(err error) Handler {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError(err.Error()), nil
	}
}

// NotImplementedHandler returns a handler that indicates a feature is not implemented
func NotImplementedHandler() Handler {
	return ErrorHandler(errors.New("feature not implemented"))
}
