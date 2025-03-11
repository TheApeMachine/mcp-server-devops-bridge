// Package memory provides interfaces and implementations for memory storage
package memory

import (
	"context"
)

// VectorStore defines the interface for vector database operations
type VectorStore interface {
	// Store saves a text document with metadata to the vector store
	Store(ctx context.Context, text string, metadata map[string]interface{}) error

	// Search performs a semantic search for similar texts
	Search(ctx context.Context, query string) ([]string, error)
}

// GraphStore defines the interface for graph database operations
type GraphStore interface {
	// Execute runs a Cypher query with parameters
	Execute(ctx context.Context, query string, params map[string]interface{}) error

	// Query searches the graph database with keywords or a custom Cypher query
	Query(ctx context.Context, keywords string, cypher string) ([]string, error)
}
