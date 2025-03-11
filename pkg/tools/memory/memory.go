// Package memory provides the memory tool implementation
package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	memstore "github.com/theapemachine/mcp-server-devops-bridge/pkg/memory"
)

// Tool implements the memory management tool
type Tool struct {
	handle      mcp.Tool
	vectorStore memstore.VectorStore
	graphStore  memstore.GraphStore
}

// New creates a new memory tool instance
func New(vectorStore memstore.VectorStore, graphStore memstore.GraphStore) *Tool {
	tool := &Tool{
		handle: mcp.NewTool(
			"memory",
			mcp.WithDescription("Manage and search memory"),
			mcp.WithString(
				"operation",
				mcp.Required(),
				mcp.Description("The operation to perform (add, query)"),
			),
			mcp.WithString(
				"document",
				mcp.Description("A longer, connected piece of unstructured text to store"),
			),
			mcp.WithString(
				"question",
				mcp.Description("A question to search the vector memory with"),
			),
			mcp.WithString(
				"keywords",
				mcp.Description("Comma-separated keywords to search the graph memory with"),
			),
			mcp.WithString(
				"cypher",
				mcp.Description("A specific Cypher query to run against the graph memory"),
			),
		),
		vectorStore: vectorStore,
		graphStore:  graphStore,
	}

	return tool
}

// Handle returns the MCP tool definition
func (tool *Tool) Handle() mcp.Tool {
	return tool.handle
}

// validate checks if the request contains valid parameters
func (tool *Tool) validate(request mcp.CallToolRequest) (ok bool, err error) {
	var op string

	if op, ok = request.Params.Arguments["operation"].(string); !ok {
		return false, fmt.Errorf("operation is required")
	}

	has := make(map[string]bool)

	for _, ctx := range []string{"document", "question", "keywords", "cypher"} {
		if _, ok = request.Params.Arguments[ctx].(string); ok {
			has[ctx] = true
		}
	}

	if op == "add" && !has["document"] && !has["cypher"] {
		return false, fmt.Errorf("at least one of document or cypher is required")
	}

	if op == "query" && !has["question"] && !has["keywords"] && !has["cypher"] {
		return false, fmt.Errorf("at least one of question, keywords, or cypher is required")
	}

	return true, nil
}

// Handler processes memory tool requests
func (tool *Tool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		ok  bool
		err error
	)

	if ok, err = tool.validate(request); !ok {
		return mcp.NewToolResultError(err.Error()), nil
	}

	switch op := request.Params.Arguments["operation"].(string); op {
	case "add":
		document := ""
		cypher := ""

		if doc, ok := request.Params.Arguments["document"].(string); ok {
			document = doc
		}

		if cyp, ok := request.Params.Arguments["cypher"].(string); ok {
			cypher = cyp
		}

		return tool.handleAddMemory(ctx, document, cypher)
	case "query":
		question := ""
		keywords := ""
		cypher := ""

		if q, ok := request.Params.Arguments["question"].(string); ok {
			question = q
		}

		if k, ok := request.Params.Arguments["keywords"].(string); ok {
			keywords = k
		}

		if cyp, ok := request.Params.Arguments["cypher"].(string); ok {
			cypher = cyp
		}

		return tool.handleQueryMemory(ctx, question, keywords, cypher)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid operation: %s", op)), nil
	}
}

// handleAddMemory processes memory addition requests
func (tool *Tool) handleAddMemory(ctx context.Context, document string, cypher string) (*mcp.CallToolResult, error) {
	errors := []string{}
	results := []string{}

	if document != "" {
		// Store the document in the vector store
		err := tool.vectorStore.Store(ctx, document, map[string]interface{}{
			"timestamp": ctx.Value("timestamp"),
			"source":    "memory_tool",
		})

		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to store in vector DB: %v", err))
		} else {
			results = append(results, "Memory added to vector store")
		}
	}

	if cypher != "" {
		// Execute the Cypher query in the graph store
		err := tool.graphStore.Execute(ctx, cypher, nil)

		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to store in graph DB: %v", err))
		} else {
			results = append(results, "Memory added to graph store")
		}
	}

	if len(errors) > 0 {
		return mcp.NewToolResultText(strings.Join(errors, "\n")), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// handleQueryMemory processes memory query requests
func (tool *Tool) handleQueryMemory(ctx context.Context, question string, keywords string, cypher string) (*mcp.CallToolResult, error) {
	errors := []string{}
	results := []string{}

	if question != "" {
		// Search the vector store
		vectorResults, err := tool.vectorStore.Search(ctx, question)

		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to query vector DB: %v", err))
		} else if len(vectorResults) > 0 {
			results = append(results, "<VECTOR_MEMORIES>")
			for _, result := range vectorResults {
				results = append(results, "\t"+result)
			}
			results = append(results, "</VECTOR_MEMORIES>")
		} else {
			results = append(results, "No vector memories found")
		}
	}

	if keywords != "" || cypher != "" {
		// Query the graph store
		graphResults, err := tool.graphStore.Query(ctx, keywords, cypher)

		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to query graph DB: %v", err))
		} else if len(graphResults) > 0 {
			results = append(results, "<GRAPH_MEMORIES>")
			for _, result := range graphResults {
				results = append(results, "\t"+result)
			}
			results = append(results, "</GRAPH_MEMORIES>")
		} else {
			results = append(results, "No graph memories found")
		}
	}

	if len(errors) > 0 {
		return mcp.NewToolResultText(strings.Join(errors, "\n")), nil
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}
