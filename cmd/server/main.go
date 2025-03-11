// Command server is the main entry point for the MCP MultiTool Server
package main

import (
	"log"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/config"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools"
)

func main() {
	// Initialize logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(&logWriter{})
	log.Println("Starting MCP MultiTool Server...")

	// Load configuration
	cfg := config.Load()
	err := cfg.Validate()
	if err != nil {
		log.Printf("Configuration warning: %v", err)
	}

	// Initialize MCP server
	mcpServer := server.NewMCPServer(
		"MCP MultiTool Server",
		"1.0.0",
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
	)

	// Initialize tool registry
	_ = NewToolRegistry(mcpServer)

	// Register tools (will be moved to separate initialization functions)
	// registry.RegisterTool("memory", memoryTool)
	// registry.RegisterTool("work_item", workItemTool)
	// registry.RegisterTool("wiki", wikiTool)
	// registry.RegisterTool("slack", slackTool)
	// etc.

	// Start the server
	log.Println("Server started, waiting for requests...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}

	log.Println("Server shutdown complete")
}

// ToolRegistry manages tool registration and lifecycle
type ToolRegistry struct {
	server *server.MCPServer
	tools  map[string]tools.Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(mcpServer *server.MCPServer) *ToolRegistry {
	return &ToolRegistry{
		server: mcpServer,
		tools:  make(map[string]tools.Tool),
	}
}

// RegisterTool registers a tool with the server
func (r *ToolRegistry) RegisterTool(name string, tool tools.Tool) {
	r.tools[name] = tool
	r.server.AddTool(tool.Handle(), tool.Handler)
}

// logWriter filters log messages
type logWriter struct{}

// Write implements io.Writer and filters some log messages
func (w *logWriter) Write(bytes []byte) (int, error) {
	// Skip logging "Prompts not supported" errors
	if strings.Contains(string(bytes), "Prompts not supported") {
		return len(bytes), nil
	}
	return os.Stdout.Write(bytes)
}
