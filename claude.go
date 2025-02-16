package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ClaudeMemoryWrapper struct {
	ctx    context.Context
	server *server.MCPServer
	qdrant *Qdrant
	neo4j  *Neo4j
}

// Initialize creates a new ClaudeMemoryWrapper with memory stores
func NewClaudeMemoryWrapper(ctx context.Context, server *server.MCPServer) (*ClaudeMemoryWrapper, error) {
	// Initialize Qdrant for semantic search
	qdrant := NewQdrant("memories", 1536) // OpenAI embedding dimension

	// Initialize Neo4j for relationship graphs
	neo4j, err := NewNeo4j()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Neo4j: %w", err)
	}

	return &ClaudeMemoryWrapper{
		ctx:    ctx,
		server: server,
		qdrant: qdrant,
		neo4j:  neo4j,
	}, nil
}

// ProcessMessage handles all Claude interactions, ensuring memory usage
func (c *ClaudeMemoryWrapper) ProcessMessage(userMessage string) (string, error) {
	// 1. Query for relevant memories
	var queryResults []map[string]interface{}

	// Try semantic search
	qdrantClient := NewQdrant("memories", 1536)
	vectorResults, err := qdrantClient.Query(userMessage)
	if err == nil {
		queryResults = append(queryResults, vectorResults...)
	}

	// Try graph relationships
	neo4jClient, err := NewNeo4j()
	if err == nil {
		defer neo4jClient.Close()
		// Query for related conversations
		graphResults, err := neo4jClient.Query(`
			MATCH (m:Memory)
			WHERE m.content CONTAINS $query
			RETURN m.content as content, m.source as source, m.timestamp as timestamp
			LIMIT 5
		`)
		if err == nil {
			queryResults = append(queryResults, graphResults...)
		}
	}

	// Format memories as context
	var memoryContext string
	if len(queryResults) > 0 {
		var memories []string
		for _, result := range queryResults {
			memories = append(memories, fmt.Sprintf(
				"Memory from %s:\n%s\n",
				result["metadata"].(map[string]interface{})["timestamp"],
				result["content"],
			))
		}
		memoryContext = "Relevant memories from our previous conversations:\n\n" + strings.Join(memories, "\n---\n") + "\n\n"
	}

	// 2. Create augmented prompt with system instructions
	augmentedPrompt := fmt.Sprintf(`%s
You are an AI assistant with access to previous conversation memories. Always consider these memories when responding.
After each response, you should identify any important information worth remembering.

Current conversation:
User: %s`, memoryContext, userMessage)

	// 3. Get Claude's response
	jsonMsg, _ := json.Marshal(map[string]interface{}{
		"message": augmentedPrompt,
	})
	rpcMsg := c.server.HandleMessage(c.ctx, jsonMsg)

	// Extract response
	rpcResponse, ok := rpcMsg.(mcp.JSONRPCResponse)
	if !ok {
		return "", fmt.Errorf("unexpected response type")
	}

	var response struct {
		Result string `json:"result"`
	}
	resultBytes, err := json.Marshal(rpcResponse.Result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	claudeResponse := response.Result

	// 4. Store the interaction as a new memory
	interaction := fmt.Sprintf("User: %s\n\nAssistant: %s", userMessage, claudeResponse)

	// Store in vector database
	result := qdrantClient.Add([]string{interaction}, map[string]interface{}{
		"source":    "conversation",
		"timestamp": time.Now(),
	})
	if result != "memory saved in vector store" {
		return claudeResponse, fmt.Errorf("failed to store memory: %s", result)
	}

	// Store in graph database with timestamp
	if neo4jClient != nil {
		timestamp := time.Now().Format(time.RFC3339)
		cypher := fmt.Sprintf(`
			CREATE (m:Memory {
				content: $content,
				source: 'conversation',
				timestamp: '%s'
			})
		`, timestamp)

		err = neo4jClient.Write(cypher)
		if err != nil {
			return claudeResponse, fmt.Errorf("failed to store in graph: %w", err)
		}
	}

	return claudeResponse, nil
}

// Helper methods for memory management
func (c *ClaudeMemoryWrapper) retrieveMemories(query string) ([]Document, error) {
	// Search vector store for semantically similar memories
	results, err := c.qdrant.Query(query)
	if err != nil {
		return nil, err
	}

	// Convert results to Documents
	var docs []Document
	for _, result := range results {
		docs = append(docs, Document{
			Text: result["content"].(string),
			Metadata: Metadata{
				Source: "conversation",
			},
		})
	}

	return docs, nil
}

func (c *ClaudeMemoryWrapper) storeMemory(text string) error {
	// Store in vector database for semantic search
	result := c.qdrant.Add([]string{text}, map[string]interface{}{
		"source":    "conversation",
		"timestamp": time.Now(),
	})
	if result != "memory saved in vector store" {
		return fmt.Errorf("failed to store in vector db: %s", result)
	}

	// Store in graph database for relationship tracking
	// You can extract entities and relationships here
	return nil
}
