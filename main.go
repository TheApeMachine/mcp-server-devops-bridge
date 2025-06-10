package main

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/agents"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/azure"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/slack"
)

// MultiTool manages all available tools
type MultiTool struct {
	tools     map[string]core.Tool
	mcpServer *server.MCPServer
}

func (mt *MultiTool) addTool(name string, tool core.Tool) {
	if tool == nil {
		return
	}
	mt.tools[name] = tool
	mt.mcpServer.AddTool(tool.Handle(), tool.Handler)
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
		tools:     make(map[string]core.Tool),
		mcpServer: mcpServer,
	}
}

func main() {
	// // Initialize memory stores
	// var vectorStore memory.VectorStore
	// var graphStore memory.GraphStore

	// // Try to initialize vector store
	// vs, err := memory.NewQdrantStore("memories")
	// if err != nil {
	//
	// } else {
	// 	vectorStore = vs
	//
	// }

	// // Try to initialize graph store
	// neo4jUrl := os.Getenv("NEO4J_URL")
	// neo4jUser := os.Getenv("NEO4J_USER")
	// neo4jPass := os.Getenv("NEO4J_PASSWORD")
	// neo4jDb := os.Getenv("NEO4J_DATABASE")

	// if neo4jUrl != "" && neo4jUser != "" && neo4jPass != "" {
	// 	gs, err := memory.NewNeo4jStore(neo4jUrl, neo4jUser, neo4jPass, neo4jDb)
	// 	if err != nil {
	//
	// 	} else {
	// 		graphStore = gs
	//
	// 	}
	// } else {
	//
	// }

	// // Initialize memory tool with both stores
	// memTool := memoryTool.New(vectorStore, graphStore)
	// multiTool.addTool("memory", memTool, vectorStore, graphStore)

	// // Initialize AI tools
	// for _, tool := range ai.RegisterAITools() {
	// 	multiTool.addTool(tool.Handle().Name, tool, vectorStore, graphStore)
	// }

	// // Initialize browser tool
	// multiTool.addTool("browser", browser.NewBrowserTool(), vectorStore, graphStore)

	// Initialize Agent tools
	agentProvider, err := agents.NewAgentProvider()
	if err == nil && agentProvider != nil {
		if len(agentProvider.Tools) > 0 {
			for name, tool := range agentProvider.Tools {
				multiTool.addTool(name, tool)
			}
		}
	}

	// Initialize Azure tools
	azureProvider := azure.NewAzureProvider()
	if len(azureProvider.Tools) > 0 {
		for name, tool := range azureProvider.Tools {
			multiTool.addTool(name, tool)
		}
	}

	// Initialize Slack tool
	slackTool := slack.NewSlackPostMessageTool()
	if slackTool != nil {
		multiTool.addTool(slackTool.Handle().Name, slackTool)
	}

	// // Start agent cleanup goroutine
	// ai.StartAgentCleanup()

	if err := server.ServeStdio(mcpServer); err != nil {
		panic(err)
	}
}
