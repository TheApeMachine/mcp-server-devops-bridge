package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
		GithubToken:   os.Getenv("GITHUB_PAT"),
		SlackToken:    os.Getenv("SLACK_BOT_TOKEN"),
		N8NBaseURL:    os.Getenv("N8N_BASE_URL"),
		N8NAPIKey:     os.Getenv("N8N_API_KEY"),
		OpenAIToken:   os.Getenv("OPENAI_API_KEY"),
		QdrantURL:     os.Getenv("QDRANT_URL"),
		QdrantAPIKey:  os.Getenv("QDRANT_API_KEY"),
		Neo4jURL:      os.Getenv("NEO4J_URL"),
		Neo4jUser:     os.Getenv("NEO4J_USER"),
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

	// Add Email tools
	addEmailTools(s)

	// Add Browser tools
	addBrowserTools(s)

	// Add Code Analysis tools
	addCodeAnalysisTools(s)

	// Add agent management tools
	addAgentManagementTools(s)

	// Start agent cleanup routine
	StartAgentCleanup()

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

var messages = []openai.ChatCompletionMessageParamUnion{
	openai.SystemMessage("You are a helpful assistant that can discuss topics with the user. Please make sure that besides providing just answers, you also keep the discussion going by asking follow-up questions."),
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
		messages = append(messages, openai.UserMessage(message))

		llm := openai.NewClient()
		chat, err := llm.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: openai.F(messages),
			Model:    openai.F(openai.ChatModelGPT4o),
		})

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get GPT-4 response: %v", err)), nil
		}

		messages = append(messages, openai.AssistantMessage(chat.Choices[0].Message.Content))

		return mcp.NewToolResultText(chat.Choices[0].Message.Content), nil
	})

	agentTool := mcp.NewTool("agent",
		mcp.WithDescription("Create a new agent to delegate tasks to"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("ID of the agent"),
		),
		mcp.WithString("system_prompt",
			mcp.Required(),
			mcp.Description("System prompt for the agent, which will define its behavior"),
		),
		mcp.WithString("task",
			mcp.Required(),
			mcp.Description("Task to delegate to the agent"),
		),
		mcp.WithString("paths",
			mcp.Description("System paths the agent should have access to"),
		),
		mcp.WithString("tools",
			mcp.Description("System commands the agent is allowed to use"),
		),
	)

	s.AddTool(agentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Create a list of allowed commands from the tools parameter
		var cmds []string
		if toolsStr, ok := request.Params.Arguments["tools"].(string); ok && toolsStr != "" {
			cmds = strings.Split(toolsStr, ",")
			for i, cmd := range cmds {
				cmds[i] = strings.TrimSpace(cmd)
			}
		}

		// Create and run the agent
		agent := NewAgent(
			request.Params.Arguments["id"].(string),
			request.Params.Arguments["system_prompt"].(string),
			request.Params.Arguments["task"].(string),
			getOptionalStringArg(request.Params.Arguments, "paths", ""),
			getOptionalStringArg(request.Params.Arguments, "tools", ""),
			cmds,
		)

		// Run the agent and get the response
		response, err := agent.Run()
		if err != nil {
			return nil, err
		}

		// Return the agent's response
		return mcp.NewToolResultText(response), nil
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

func addCodeAnalysisTools(s *server.MCPServer) {
	// Initialize code analyzer
	analyzer, err := NewCodeAnalyzer()
	if err != nil {
		log.Printf("Warning: Failed to initialize code analyzer: %v", err)
		return
	}

	// Analyze Code Tool
	analyzeCodeTool := mcp.NewTool("analyze_code",
		mcp.WithDescription("Analyze code for complexity, potential bugs, and security issues"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the code file to analyze"),
		),
	)

	s.AddTool(analyzeCodeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := request.Params.Arguments["path"].(string)

		analysis, err := analyzer.AnalyzeCode(ctx, path)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to analyze code: %v", err)), nil
		}

		// Format results
		result := fmt.Sprintf("Code Analysis Results for %s:\n\n", path)
		result += fmt.Sprintf("Complexity Score: %.2f\n", analysis.Complexity)
		result += fmt.Sprintf("Bug Probability: %.2f%%\n", analysis.BugProbability*100)

		if len(analysis.Security) > 0 {
			result += "\nSecurity Issues:\n"
			for issue, description := range analysis.Security {
				result += fmt.Sprintf("- %s: %s\n", issue, description)
			}
		} else {
			result += "\nNo security issues found."
		}

		return mcp.NewToolResultText(result), nil
	})

	// Store Code Context Tool
	storeContextTool := mcp.NewTool("store_code_context",
		mcp.WithDescription("Store code context information for future reference"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the code file"),
		),
		mcp.WithString("language",
			mcp.Required(),
			mcp.Description("Programming language of the code"),
		),
		mcp.WithString("framework",
			mcp.Description("Framework used, if any"),
		),
	)

	s.AddTool(storeContextTool, handleStoreCodeContext)

	// Query Similar Code Tool
	querySimilarTool := mcp.NewTool("query_similar_code",
		mcp.WithDescription("Find similar code based on context and relationships"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the code file to find similar contexts for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of similar contexts to return"),
		),
	)

	s.AddTool(querySimilarTool, handleQuerySimilarCode)
}

func addEmailTools(s *server.MCPServer) {
	// Get Inbox Tool
	getInboxTool := mcp.NewTool("get_inbox",
		mcp.WithDescription("Retrieve emails from Outlook inbox"),
	)
	s.AddTool(getInboxTool, handleGetInbox)

	// Search Emails Tool
	searchEmailsTool := mcp.NewTool("search_emails",
		mcp.WithDescription("Search through emails using a query string"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query to find specific emails"),
		),
	)
	s.AddTool(searchEmailsTool, handleSearchEmails)

	// Reply to Email Tool
	replyToEmailTool := mcp.NewTool("reply_to_email",
		mcp.WithDescription("Send a reply to a specific email"),
		mcp.WithString("message_id",
			mcp.Required(),
			mcp.Description("ID of the message to reply to"),
		),
		mcp.WithString("reply_message",
			mcp.Required(),
			mcp.Description("Message content for the reply"),
		),
	)
	s.AddTool(replyToEmailTool, handleReplyToEmail)
}

// Add agent management tools
func addAgentManagementTools(s *server.MCPServer) {
	// 1. Start Agent Tool
	startAgentTool := mcp.NewTool(
		"start_agent",
		mcp.WithDescription("Start a new agent process in the background"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Unique identifier for the agent"),
		),
		mcp.WithString("system_prompt",
			mcp.Required(),
			mcp.Description("System prompt for the agent"),
		),
		mcp.WithString("initial_task",
			mcp.Required(),
			mcp.Description("Initial task for the agent"),
		),
		mcp.WithString("paths",
			mcp.Description("System paths the agent should have access to"),
		),
		mcp.WithString("tools",
			mcp.Description("System commands the agent is allowed to use"),
		),
	)

	s.AddTool(startAgentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agentID := request.Params.Arguments["id"].(string)
		systemPrompt := request.Params.Arguments["system_prompt"].(string)
		initialTask := request.Params.Arguments["initial_task"].(string)
		paths := getOptionalStringArg(request.Params.Arguments, "paths", "")
		tools := getOptionalStringArg(request.Params.Arguments, "tools", "")

		// Check if agent already exists
		agentsMutex.RLock()
		_, exists := runningAgents[agentID]
		agentsMutex.RUnlock()

		if exists {
			return mcp.NewToolResultText(fmt.Sprintf("Agent with ID %s already exists", agentID)), nil
		}

		// Parse allowed commands
		var cmds []string
		if tools != "" {
			cmds = strings.Split(tools, ",")
			for i, cmd := range cmds {
				cmds[i] = strings.TrimSpace(cmd)
			}
		}

		// Create agent
		agent := NewAgent(agentID, systemPrompt, initialTask, paths, tools, cmds)

		// Create channels for communication
		commandChan := make(chan string, 10)
		responseChan := make(chan string, 10)
		killChan := make(chan struct{})

		// Create context with cancel function
		agentCtx, cancel := context.WithCancel(context.Background())

		// Register the agent
		runningAgent := &RunningAgent{
			agent:        agent,
			commandChan:  commandChan,
			responseChan: responseChan,
			killChan:     killChan,
			ctx:          agentCtx,
			cancel:       cancel,
			lastActive:   time.Now(),
		}

		agentsMutex.Lock()
		runningAgents[agentID] = runningAgent
		agentsMutex.Unlock()

		// Start the agent loop in a goroutine
		go StartAgentLoop(agentID, agent, commandChan, responseChan, killChan, agentCtx)

		// Send initial task
		commandChan <- initialTask

		// Wait for initial response
		var response string
		select {
		case response = <-responseChan:
			// Got response
		case <-time.After(30 * time.Second):
			response = "Agent started but timed out waiting for initial response"
		}

		return mcp.NewToolResultText(fmt.Sprintf("Agent %s started successfully.\nInitial response: %s", agentID, response)), nil
	})

	// 2. Send Command to Agent Tool
	sendCommandTool := mcp.NewTool(
		"send_command",
		mcp.WithDescription("Send a command to a running agent"),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("ID of the agent to send command to"),
		),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("Command to send to the agent"),
		),
	)

	s.AddTool(sendCommandTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agentID := request.Params.Arguments["agent_id"].(string)
		command := request.Params.Arguments["command"].(string)

		// Get the agent
		agentsMutex.RLock()
		runningAgent, exists := runningAgents[agentID]
		agentsMutex.RUnlock()

		if !exists {
			return mcp.NewToolResultText(fmt.Sprintf("Agent with ID %s not found", agentID)), nil
		}

		// Send command
		runningAgent.commandChan <- command

		// Wait for response
		var response string
		select {
		case response = <-runningAgent.responseChan:
			// Got response
		case <-time.After(30 * time.Second):
			return mcp.NewToolResultText("Timed out waiting for agent response"), nil
		}

		return mcp.NewToolResultText(response), nil
	})

	// 3. Kill Agent Tool
	killAgentTool := mcp.NewTool(
		"kill_agent",
		mcp.WithDescription("Terminate a running agent"),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("ID of the agent to terminate"),
		),
	)

	s.AddTool(killAgentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agentID := request.Params.Arguments["agent_id"].(string)

		// Get the agent
		agentsMutex.RLock()
		runningAgent, exists := runningAgents[agentID]
		agentsMutex.RUnlock()

		if !exists {
			return mcp.NewToolResultText(fmt.Sprintf("Agent with ID %s not found", agentID)), nil
		}

		// Send kill signal
		close(runningAgent.killChan)

		// Cancel context
		runningAgent.cancel()

		// Remove from registry
		agentsMutex.Lock()
		delete(runningAgents, agentID)
		agentsMutex.Unlock()

		return mcp.NewToolResultText(fmt.Sprintf("Agent %s terminated successfully", agentID)), nil
	})

	// 4. List Agents Tool
	listAgentsTool := mcp.NewTool(
		"list_agents",
		mcp.WithDescription("List all running agents"),
	)

	s.AddTool(listAgentsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agentsMutex.RLock()
		defer agentsMutex.RUnlock()

		if len(runningAgents) == 0 {
			return mcp.NewToolResultText("No agents currently running"), nil
		}

		var results []string
		for id, agent := range runningAgents {
			idleTime := time.Since(agent.lastActive).Round(time.Second)
			promptPreview := agent.agent.SystemPrompt
			if len(promptPreview) > 50 {
				promptPreview = promptPreview[:50] + "..."
			}
			results = append(results, fmt.Sprintf("ID: %s\nIdle: %s\nSystem: %s\n---",
				id, idleTime, promptPreview))
		}

		return mcp.NewToolResultText(strings.Join(results, "\n")), nil
	})

	// 5. Subscribe Agent to Topic
	subscribeAgentTool := mcp.NewTool(
		"subscribe_agent",
		mcp.WithDescription("Subscribe an agent to a message topic"),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("ID of the agent"),
		),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("Topic to subscribe to"),
		),
	)

	s.AddTool(subscribeAgentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agentID := request.Params.Arguments["agent_id"].(string)
		topic := request.Params.Arguments["topic"].(string)

		// Check if agent exists
		agentsMutex.RLock()
		_, exists := runningAgents[agentID]
		agentsMutex.RUnlock()

		if !exists {
			return mcp.NewToolResultText(fmt.Sprintf("Agent with ID %s not found", agentID)), nil
		}

		// Subscribe to topic
		err := messageBus.Subscribe(agentID, topic)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Failed to subscribe: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Agent %s subscribed to topic %s", agentID, topic)), nil
	})

	// 6. Send Message Tool (for agent-to-agent communication)
	sendMessageTool := mcp.NewTool(
		"send_agent_message",
		mcp.WithDescription("Send a message to a topic for agent-to-agent communication"),
		mcp.WithString("topic",
			mcp.Required(),
			mcp.Description("Topic to send message to"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Message content"),
		),
	)

	s.AddTool(sendMessageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		topic := request.Params.Arguments["topic"].(string)
		content := request.Params.Arguments["content"].(string)

		// Create and publish message
		msg := Message{
			From:    "mcp-server", // Message from MCP server itself
			Topic:   topic,
			Content: content,
		}

		err := messageBus.Publish(msg)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Failed to send message: %v", err)), nil
		}

		return mcp.NewToolResultText("Message sent successfully to topic: " + topic), nil
	})
}

func getOptionalStringArg(args map[string]interface{}, key string, defaultValue string) string {
	if val, ok := args[key]; ok {
		return val.(string)
	}
	return defaultValue
}
