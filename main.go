package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/core/middleware"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/memory"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/ai"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/azure"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/browser"
	memoryTool "github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/memory"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/slack"
)

// MultiTool manages all available tools
type MultiTool struct {
	tools map[string]core.Tool
}

func (mt *MultiTool) addTool(name string, tool core.Tool, vectorStore memory.VectorStore, graphStore memory.GraphStore) {
	mt.tools[name] = tool
	// Wrap handler with memory middleware
	handler := middleware.MemoryMiddleware(tool.Handler, vectorStore, graphStore)
	mcpServer.AddTool(tool.Handle(), handler)
}

var (
	mcpServer *server.MCPServer
	multiTool MultiTool
)

func init() {
	mcpServer = server.NewMCPServer(
		"Multi-Tool MCP Server",
		"1.0.0",
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
	)

	multiTool = MultiTool{
		tools: make(map[string]core.Tool),
	}
}

func main() {
	// Initialize memory stores
	var vectorStore memory.VectorStore
	var graphStore memory.GraphStore

	// Try to initialize vector store
	vs, err := memory.NewQdrantStore("memories")
	if err != nil {
		log.Printf("Warning: Could not initialize vector store: %v", err)
	} else {
		vectorStore = vs
		log.Println("Vector store initialized successfully")
	}

	// Try to initialize graph store
	neo4jUrl := os.Getenv("NEO4J_URL")
	neo4jUser := os.Getenv("NEO4J_USER")
	neo4jPass := os.Getenv("NEO4J_PASSWORD")
	neo4jDb := os.Getenv("NEO4J_DATABASE")

	if neo4jUrl != "" && neo4jUser != "" && neo4jPass != "" {
		gs, err := memory.NewNeo4jStore(neo4jUrl, neo4jUser, neo4jPass, neo4jDb)
		if err != nil {
			log.Printf("Warning: Could not initialize graph store: %v", err)
		} else {
			graphStore = gs
			log.Println("Graph store initialized successfully")
		}
	} else {
		log.Println("Warning: Neo4j environment variables not set, graph store not initialized")
	}

	// Initialize memory tool with both stores
	memTool := memoryTool.New(vectorStore, graphStore)
	multiTool.addTool("memory", memTool, vectorStore, graphStore)

	// Initialize AI tools
	for _, tool := range ai.RegisterAITools() {
		multiTool.addTool(tool.Handle().Name, tool, vectorStore, graphStore)
	}

	// Initialize browser tool
	multiTool.addTool("browser", browser.NewBrowserTool(), vectorStore, graphStore)

	// Initialize Azure tools
	azureProvider := azure.NewAzureProvider()
	for name, tool := range azureProvider.Tools {
		multiTool.addTool(name, tool, vectorStore, graphStore)
	}

	// Initialize Slack tool
	multiTool.addTool("slack", slack.NewSlackTool(), vectorStore, graphStore)

	// Start agent cleanup goroutine
	ai.StartAgentCleanup()

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}
