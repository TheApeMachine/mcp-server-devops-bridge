package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/wiki"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/openai/openai-go"
)

// AzureDevOpsConfig holds the configuration for Azure DevOps connection
type AzureDevOpsConfig struct {
	OrganizationURL     string
	PersonalAccessToken string
	Project             string
}

// Global clients and config
var (
	connection     *azuredevops.Connection
	workItemClient workitemtracking.Client
	wikiClient     wiki.Client
	coreClient     core.Client
	config         AzureDevOpsConfig
	intConfig      IntegrationsConfig
)

func main() {
	// Load configuration from environment variables
	config = AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + os.Getenv("AZURE_DEVOPS_ORG"),
		PersonalAccessToken: os.Getenv("AZDO_PAT"),
		Project:             os.Getenv("AZURE_DEVOPS_PROJECT"),
	}

	intConfig = IntegrationsConfig{
		GithubToken: os.Getenv("GITHUB_PAT"),
		SlackToken:  os.Getenv("SLACK_BOT_TOKEN"),
		N8NBaseURL:  os.Getenv("N8N_BASE_URL"),
		N8NAPIKey:   os.Getenv("N8N_API_KEY"),
		OpenAIToken:  os.Getenv("OPENAI_API_KEY"),
		QdrantURL:    os.Getenv("QDRANT_URL"),
		QdrantAPIKey: os.Getenv("QDRANT_API_KEY"),
		Neo4jURL:     os.Getenv("NEO4J_URL"),
		Neo4jUser:    os.Getenv("NEO4J_USER"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
	}

	// Validate configuration
	if config.OrganizationURL == "" || config.PersonalAccessToken == "" || config.Project == "" {
		log.Fatal("Missing required environment variables: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT")
	}

	// Initialize Azure DevOps clients
	if err := initializeClients(config); err != nil {
		log.Fatalf("Failed to initialize Azure DevOps clients: %v", err)
	}

	// Initialize integration clients
	if err := initializeIntegrationClients(intConfig); err != nil {
		log.Printf("Warning: Failed to initialize integration clients: %v", err)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"Azure DevOps MCP Server",
		"1.0.0",
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
	)

	// Configure custom error handling
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(&logWriter{})

	// Add Work Item tools
	addWorkItemTools(s)

	// Add Wiki tools
	addWikiTools(s)

	// Add Integration tools
	addIntegrationTools(s)

	// Add Claude tools
	addClaudeTools(s)

	// Add OpenAI tools
	addOpenAITools(s)

	// Add Memory tools
	addMemoryTools(s)

	// Start the server
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtr(s string) *string {
	return &s
}

// Initialize Azure DevOps clients
func initializeClients(config AzureDevOpsConfig) error {
	connection = azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	ctx := context.Background()

	var err error

	// Initialize Work Item Tracking client
	workItemClient, err = workitemtracking.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create work item client: %v", err)
	}

	// Initialize Wiki client
	wikiClient, err = wiki.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create wiki client: %v", err)
	}

	// Initialize Core client
	coreClient, err = core.NewClient(ctx, connection)
	if err != nil {
		return fmt.Errorf("failed to create core client: %v", err)
	}

	return nil
}

type logWriter struct{}

func (w *logWriter) Write(bytes []byte) (int, error) {
	// Skip logging "Prompts not supported" errors
	if strings.Contains(string(bytes), "Prompts not supported") {
		return len(bytes), nil
	}
	return fmt.Print(string(bytes))
}

func addClaudeTools(s *server.MCPServer) {
	// Simple Claude chat without any integrations
	claudeTool := mcp.NewTool("claude_chat",
		mcp.WithDescription("Chat with Claude"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("Message to send to Claude"),
		),
	)

	s.AddTool(claudeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message := request.Params.Arguments["message"].(string)
		return mcp.NewToolResultText(message), nil // For now, just echo the message
	})
}

func addOpenAITools(s *server.MCPServer) {
	openAITool := mcp.NewTool("discuss_with_gpt4",
		mcp.WithDescription("Discuss a topic with GPT-4 to get additional perspectives"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("Message to discuss with GPT-4"),
		),
	)

	s.AddTool(openAITool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message := request.Params.Arguments["message"].(string)

		llm := openai.NewClient()
		chat, err := llm.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage("You are a helpful assistant that can discuss topics with the user."),
				openai.UserMessage(message),
			}),
			Model: openai.F(openai.ChatModelGPT4o),
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get GPT-4 response: %v", err)), nil
		}

		return mcp.NewToolResultText(chat.Choices[0].Message.Content), nil
	})
}

func addMemoryTools(s *server.MCPServer) {
	// Add Memory
	addMemoryTool := mcp.NewTool("add_memory",
		mcp.WithDescription("Store a new memory in both vector and graph stores"),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("The text content to remember"),
		),
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("Source of the memory (e.g., 'conversation', 'work_item', etc)"),
		),
		mcp.WithString("cypher",
			mcp.Description("Optional Cypher query to create relationships"),
		),
	)
	s.AddTool(addMemoryTool, handleAddMemory)

	// Query Memory
	queryMemoryTool := mcp.NewTool("query_memory",
		mcp.WithDescription("Search through stored memories"),
		mcp.WithString("semantic_search",
			mcp.Description("Natural language search query"),
		),
		mcp.WithString("keywords",
			mcp.Description("Comma-separated keywords to look for"),
		),
		mcp.WithString("graph_query",
			mcp.Description("Cypher query to find related memories"),
		),
	)
	s.AddTool(queryMemoryTool, handleQueryMemory)
}
