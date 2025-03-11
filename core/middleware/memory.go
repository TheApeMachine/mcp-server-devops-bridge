// Package middleware provides middleware components for enhancing MCP tools
package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/memory"
)

// MemoryContext represents context that should be preserved for memory operations
type MemoryContext struct {
	LastQueryTime  time.Time
	RecentQueries  map[string]time.Time
	MemoryInjected bool
	RequestContext string // Captures the context of the current request
}

// MemoryOperationHistory tracks recent memory operations to prevent loops
type MemoryOperationHistory struct {
	LastQueryTime   time.Time
	RecentQueries   map[string]time.Time
	QueryThreshold  time.Duration
	MemoryInjected  bool
	ContextSnapshot string
}

// MemoryMiddleware is a middleware that automatically injects memory tools into the request.
func MemoryMiddleware(
	handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
	vectorStore memory.VectorStore,
	graphStore memory.GraphStore,
) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Only process non-memory tools to avoid loops
		if request.Params.Name == "memory" {
			return handler(ctx, request)
		}

		// Initialize memory context if needed
		memCtx := &MemoryContext{
			LastQueryTime: time.Now(),
			RecentQueries: make(map[string]time.Time),
		}

		// Extract context from the request
		requestContext := extractContext(request)
		memCtx.RequestContext = requestContext

		// Search for relevant memories
		var memories []string

		// 1. Search vector store
		if vectorStore != nil {
			vectorResults, err := vectorStore.Search(ctx, requestContext)
			if err == nil && len(vectorResults) > 0 {
				memories = append(memories, fmt.Sprintf("From vector store: %s", strings.Join(vectorResults, "; ")))
			}
		}

		// 2. Search graph store
		if graphStore != nil {
			graphResults, err := graphStore.Query(ctx, extractKeywords(requestContext), "")
			if err == nil && len(graphResults) > 0 {
				memories = append(memories, fmt.Sprintf("From graph store: %s", strings.Join(graphResults, "; ")))
			}
		}

		// Inject memories into the request context if found
		if len(memories) > 0 {
			memCtx.MemoryInjected = true

			// Create a new context with memories added
			ctxWithMemories := context.WithValue(ctx, "memories", memories)

			// Process the request with the enhanced context
			result, err := handler(ctxWithMemories, request)

			// After processing, store the result as a new memory
			if err == nil && result != nil {
				storeResult(ctx, vectorStore, graphStore, requestContext, result)
			}

			return result, err
		}

		// No memories found, just pass through
		result, err := handler(ctx, request)

		// Still store the result
		if err == nil && result != nil {
			storeResult(ctx, vectorStore, graphStore, requestContext, result)
		}

		return result, err
	}
}

// extractContext gathers queryable context from the request
func extractContext(request mcp.CallToolRequest) string {
	var contextParts []string

	// Add tool name
	contextParts = append(contextParts, fmt.Sprintf("Tool: %s", request.Params.Name))

	// Add operation if it exists
	if operation, ok := request.Params.Arguments["operation"].(string); ok {
		contextParts = append(contextParts, fmt.Sprintf("Operation: %s", operation))
	}

	// Add query if it exists
	if query, ok := request.Params.Arguments["query"].(string); ok {
		contextParts = append(contextParts, fmt.Sprintf("Query: %s", query))
	}

	// Add any ID if it exists
	if id, ok := request.Params.Arguments["id"].(float64); ok {
		contextParts = append(contextParts, fmt.Sprintf("ID: %f", id))
	}

	// Combine and return
	return strings.Join(contextParts, " | ")
}

// extractKeywords pulls keywords from a string for graph database queries
func extractKeywords(text string) string {
	// For now, use a simple approach of splitting and cleaning
	words := strings.Fields(text)
	var keywords []string

	for _, word := range words {
		// Clean the word
		word = strings.ToLower(strings.Trim(word, ".,;:!?()[]{}\"'"))

		// Skip short words, common words, etc.
		if len(word) > 3 {
			keywords = append(keywords, word)
		}
	}

	return strings.Join(keywords, ",")
}

// storeResult saves the result of a tool operation to memory
func storeResult(
	ctx context.Context,
	vectorStore memory.VectorStore,
	graphStore memory.GraphStore,
	requestContext string,
	result *mcp.CallToolResult,
) {
	// Check for a valid result
	if result == nil {
		return
	}

	// Extract the text based on the result type
	var text string

	// Check if it's an error result
	if result.IsError {
		return // Don't store error results
	}

	// Extract text from Content if available
	if len(result.Content) > 0 {
		// Try to extract the text from the first content item
		if strContent, ok := result.Content[0].(string); ok {
			text = strContent
		}
	}

	// If no text was found, try to get a string representation
	if text == "" {
		// Fallback: skip storing this result as we couldn't extract text
		return
	}

	// Store in vector database
	if vectorStore != nil {
		vectorStore.Store(ctx, text, map[string]interface{}{
			"context": requestContext,
			"time":    time.Now().Format(time.RFC3339),
		})
	}

	// Store in graph database
	if graphStore != nil {
		// Simple cypher query to create a node with the text
		cypher := fmt.Sprintf(
			"CREATE (m:Memory {text: $text, timestamp: $timestamp, context: $context})",
		)
		graphStore.Execute(ctx, cypher, map[string]interface{}{
			"text":      text,
			"timestamp": time.Now().Unix(),
			"context":   requestContext,
		})
	}
}
