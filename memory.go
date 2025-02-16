package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleAddMemory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text := request.Params.Arguments["text"].(string)
	source := request.Params.Arguments["source"].(string)
	cypher, hasCypher := request.Params.Arguments["cypher"].(string)

	// Store in vector database
	qdrantClient := NewQdrant("memories", 1536)
	result := qdrantClient.Add([]string{text}, map[string]interface{}{
		"source":    source,
		"timestamp": time.Now(),
	})
	if result != "memory saved in vector store" {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to store in vector DB: %v", result)), nil
	}

	// Store relationships in graph database if specified
	if hasCypher {
		neo4jClient, err := NewNeo4j()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to Neo4j: %v", err)), nil
		}
		defer neo4jClient.Close()

		err = neo4jClient.Write(cypher)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to store relationships: %v", err)), nil
		}
	}

	return mcp.NewToolResultText("Successfully stored memory"), nil
}

func handleQueryMemory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	semanticSearch, hasSemanticSearch := request.Params.Arguments["semantic_search"].(string)
	keywordsStr, hasKeywords := request.Params.Arguments["keywords"].(string)
	graphQuery, hasGraphQuery := request.Params.Arguments["graph_query"].(string)

	var results []map[string]interface{}

	// Semantic search
	if hasSemanticSearch {
		qdrantClient := NewQdrant("memories", 1536)
		vectorResults, err := qdrantClient.Query(semanticSearch)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed vector search: %v", err)), nil
		}
		results = append(results, vectorResults...)
	}

	// Keyword search
	if hasKeywords {
		keywords := strings.Split(keywordsStr, ",")
		qdrantClient := NewQdrant("memories", 1536)
		for _, keyword := range keywords {
			vectorResults, err := qdrantClient.Query(strings.TrimSpace(keyword))
			if err != nil {
				continue
			}
			results = append(results, vectorResults...)
		}
	}

	// Graph query
	if hasGraphQuery {
		neo4jClient, err := NewNeo4j()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to Neo4j: %v", err)), nil
		}
		defer neo4jClient.Close()

		graphResults, err := neo4jClient.Query(graphQuery)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed graph query: %v", err)), nil
		}
		results = append(results, graphResults...)
	}

	// Format results
	var formattedResults []string
	for _, result := range results {
		formattedResults = append(formattedResults, fmt.Sprintf(
			"Content: %s\nSource: %s\nTimestamp: %s\n---",
			result["content"],
			result["metadata"].(map[string]interface{})["source"],
			result["metadata"].(map[string]interface{})["timestamp"],
		))
	}

	return mcp.NewToolResultText(strings.Join(formattedResults, "\n")), nil
}
