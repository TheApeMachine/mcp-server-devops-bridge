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

		// Extract rich context from the request
		requestContext := extractContext(request)
		memCtx.RequestContext = requestContext

		// Search for relevant memories with context-aware filtering
		var memories []string
		var relevantMemories bool

		// 1. Search vector store with semantic similarity
		if vectorStore != nil {
			vectorResults, err := vectorStore.Search(ctx, requestContext)
			if err == nil && len(vectorResults) > 0 {
				// Filter and rank memories by relevance
				filteredResults := filterRelevantMemories(vectorResults, requestContext)
				if len(filteredResults) > 0 {
					memories = append(memories, fmt.Sprintf("Relevant context: %s", strings.Join(filteredResults, "; ")))
					relevantMemories = true
				}
			}
		}

		// 2. Search graph store with relationship context
		if graphStore != nil {
			keywords := extractKeywords(requestContext)
			graphResults, err := graphStore.Query(ctx, keywords, buildGraphQuery(request))
			if err == nil && len(graphResults) > 0 {
				// Enhance results with relationship context
				enhancedResults := enhanceWithRelationships(graphResults)
				if len(enhancedResults) > 0 {
					memories = append(memories, fmt.Sprintf("Related context: %s", strings.Join(enhancedResults, "; ")))
					relevantMemories = true
				}
			}
		}

		// Create context with memory metadata
		ctxWithMeta := context.WithValue(ctx, "memory_context", memCtx)

		// Inject memories into the request context if relevant ones found
		if relevantMemories {
			memCtx.MemoryInjected = true
			ctxWithMeta = context.WithValue(ctxWithMeta, "memories", memories)
		}

		// Process the request with the enhanced context
		result, err := handler(ctxWithMeta, request)

		// Store the result with enhanced context if successful
		if err == nil && result != nil {
			storeResult(ctx, vectorStore, graphStore, requestContext, result)
		}

		return result, err
	}
}

// filterRelevantMemories filters and ranks memories by relevance to the current context
func filterRelevantMemories(memories []string, context string) []string {
	// TODO: Implement semantic similarity ranking
	return memories
}

// enhanceWithRelationships adds relationship context to graph results
func enhanceWithRelationships(results []string) []string {
	// TODO: Implement relationship enhancement
	return results
}

// buildGraphQuery constructs a context-aware graph query
func buildGraphQuery(request mcp.CallToolRequest) string {
	// TODO: Implement dynamic query building
	return ""
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
