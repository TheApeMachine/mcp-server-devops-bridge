package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/memory"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/ai"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/azure"
	memoryTool "github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/memory"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/slack"
)

// MultiTool manages all available tools
type MultiTool struct {
	tools map[string]core.Tool
}

func (mt *MultiTool) addTool(name string, tool core.Tool) {
	mt.tools[name] = tool
	mcpServer.AddTool(tool.Handle(), tool.Handler)
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(&logWriter{})

	// Initialize memory stores
	vectorStore, err := memory.NewQdrantStore("memories")
	if err != nil {
		log.Printf("Warning: Could not initialize vector store: %v", err)
	}

	neo4jUrl := os.Getenv("NEO4J_URL")
	neo4jUser := os.Getenv("NEO4J_USER")
	neo4jPass := os.Getenv("NEO4J_PASSWORD")
	neo4jDb := os.Getenv("NEO4J_DATABASE")

	graphStore, err := memory.NewNeo4jStore(neo4jUrl, neo4jUser, neo4jPass, neo4jDb)
	if err != nil {
		log.Printf("Warning: Could not initialize graph store: %v", err)
	}

	// Initialize tools
	multiTool.addTool("memory", memoryTool.New(vectorStore, graphStore))
	multiTool.addTool("agent", ai.NewAgentTool())

	// Initialize Azure tools
	azureProvider := azure.NewAzureProvider()
	// Since tools is an unexported field, we need to create a helper method
	// For now, let's add each tool directly to avoid changing the original code
	for name, tool := range azureProvider.Tools {
		multiTool.addTool(name, tool)
	}

	// Initialize Slack tool
	multiTool.addTool("slack", slack.NewSlackTool())

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}

type logWriter struct{}

func (w *logWriter) Write(bytes []byte) (int, error) {
	// Skip logging "Prompts not supported" errors
	if strings.Contains(string(bytes), "Prompts not supported") {
		return len(bytes), nil
	}
	return fmt.Print(string(bytes))
}
