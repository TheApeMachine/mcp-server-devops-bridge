package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/core/container"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/system"
)

type AgentTool struct {
	handle     mcp.Tool
	agentMutex sync.RWMutex
}

func NewAgentTool() core.Tool {
	return &AgentTool{
		handle: mcp.NewTool(
			"agent",
			mcp.WithDescription("Manage agents"),
			mcp.WithString(
				"id",
				mcp.Required(),
				mcp.Description("Unique identifier for the agent"),
			),
			mcp.WithString(
				"system_prompt",
				mcp.Required(),
				mcp.Description("The system prompt for the agent, determining its personality and behavior"),
			),
			mcp.WithString(
				"task",
				mcp.Required(),
				mcp.Description("The task to be performed by the agent"),
			),
			mcp.WithString(
				"paths",
				mcp.Description("The system paths that the agent can access"),
			),
			mcp.WithString(
				"tools",
				mcp.Description("The terminal commands the agent can execute"),
			),
		),
	}
}

func (tool *AgentTool) Handle() mcp.Tool {
	return tool.handle
}

// ToOpenAITool converts the agent tool to OpenAI format
func (tool *AgentTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("agent"),
			Description: openai.String("Create a new agent to delegate tasks to"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "Unique identifier for the agent",
					},
					"system_prompt": map[string]any{
						"type":        "string",
						"description": "The system prompt for the agent, determining its personality and behavior",
					},
					"task": map[string]any{
						"type":        "string",
						"description": "The task to be performed by the agent",
					},
					"paths": map[string]any{
						"type":        "string",
						"description": "The system paths that the agent can access",
					},
					"tools": map[string]any{
						"type":        "string",
						"description": "The terminal commands the agent can execute",
					},
				},
				"required": []string{"id", "system_prompt", "task"},
			}),
		}),
	}
}

func (tool *AgentTool) Handler(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var (
		agentID      string
		ok           bool
		systemPrompt string
		task         string
		paths        string
		tools        string
	)

	if agentID, ok = request.Params.Arguments["id"].(string); !ok {
		return mcp.NewToolResultError("Agent ID is required"), nil
	}

	tool.agentMutex.RLock()
	_, exists := runningAgents[agentID]
	tool.agentMutex.RUnlock()

	if exists {
		return mcp.NewToolResultError("Agent already exists"), nil
	}

	if systemPrompt, ok = request.Params.Arguments["system_prompt"].(string); !ok {
		return mcp.NewToolResultError("System prompt is required"), nil
	}

	if task, ok = request.Params.Arguments["task"].(string); !ok {
		return mcp.NewToolResultError("Task is required"), nil
	}

	if pathsTry, ok := request.Params.Arguments["paths"].(string); ok {
		paths = pathsTry
	}

	if toolsTry, ok := request.Params.Arguments["tools"].(string); ok {
		tools = toolsTry
	}

	// Create allowed commands list from the tools string
	cmds := []string{}
	if tools != "" {
		cmds = strings.Split(tools, ",")
		// Trim whitespace from each command
		for i, cmd := range cmds {
			cmds[i] = strings.TrimSpace(cmd)
		}
	}

	agent := NewAgent(
		agentID, systemPrompt, task, paths, tools, cmds,
	)

	tool.agentMutex.Lock()
	runningAgents[agentID] = &RunningAgent{
		agent:        agent,
		commandChan:  agent.commandChan,
		responseChan: make(chan string, 10),
		killChan:     agent.killChan,
		lastActive:   time.Now(),
	}
	tool.agentMutex.Unlock()

	go agent.Run()

	return mcp.NewToolResultText(fmt.Sprintf("Agent '%s' created successfully", agentID)), nil
}

type Agent struct {
	ID           string
	SystemPrompt string
	Task         string
	Params       openai.ChatCompletionNewParams
	Paths        string
	Tools        string
	killChan     chan struct{}
	commandChan  chan string
	contextStore map[string]interface{}
	contextMutex sync.RWMutex
}

func NewAgent(id, systemPrompt, task, paths, tools string, cmds []string) *Agent {
	// Create our system tool with the allowed commands
	systemTool := system.NewSystemTool(cmds)
	// Create our messaging tool
	messagingTool := NewSendAgentMessageTool()

	// Convert tools to OpenAI format
	openaiTools := []openai.ChatCompletionToolParam{
		systemTool.ToOpenAITool(),
		messagingTool.(*SendAgentMessageTool).ToOpenAITool(),
	}

	// Enhance system prompt with tool awareness
	enhancedPrompt := fmt.Sprintf(`%s

Available Tools:
- System commands: %s
- Messaging capabilities for inter-agent communication
- Memory access for context persistence

Task Context:
Your task is: %s

Remember to:
1. Use available tools effectively
2. Maintain context between interactions
3. Communicate clearly with other agents
4. Store important information in memory
`, systemPrompt, strings.Join(cmds, ", "), task)

	return &Agent{
		ID:           id,
		SystemPrompt: enhancedPrompt,
		Task:         task,
		Params: openai.ChatCompletionNewParams{
			Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(enhancedPrompt),
				openai.UserMessage(task),
			}),
			Model:       openai.F(openai.ChatModelGPT4oMini),
			Tools:       openai.F(openaiTools),
			Temperature: openai.F(0.0),
		},
		Paths:        paths,
		Tools:        tools,
		killChan:     make(chan struct{}),
		commandChan:  make(chan string, 10),
		contextStore: make(map[string]interface{}),
	}
}

func (agent *Agent) Run() {
	client := openai.NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add agent ID to context
	ctx = context.WithValue(ctx, "agent_id", agent.ID)

	log.Printf("Agent '%s' started with task: %s", agent.ID, agent.Task)

	for {
		select {
		case <-agent.killChan:
			log.Printf("Agent '%s' terminated by kill signal", agent.ID)
			return
		case cmd := <-agent.commandChan:
			log.Printf("Agent '%s' received command: %s", agent.ID, cmd)

			// Store command in context
			agent.storeContext("last_command", cmd)

			// Execute command if it's a system command
			if strings.HasPrefix(cmd, "system:") {
				cmdStr := strings.TrimPrefix(cmd, "system:")
				output, err := executeCommand(cmdStr, agent.Paths, agent.Tools)
				if err != nil {
					log.Printf("Agent %s command error: %v", agent.ID, err)
					continue
				}
				// Store result in context
				agent.storeContext("last_result", output)
				// Update conversation with command result
				messages := agent.Params.Messages.Value
				messages = append(messages, openai.AssistantMessage(
					fmt.Sprintf("Command executed: %s\nResult: %s", cmdStr, output),
				))
				agent.Params.Messages.Value = messages
			} else {
				// Add user command to conversation
				messages := agent.Params.Messages.Value
				messages = append(messages, openai.UserMessage(cmd))
				agent.Params.Messages.Value = messages
			}

			// Get completion from OpenAI for the command
			chat, err := client.Chat.Completions.New(ctx, agent.Params)
			if err != nil {
				log.Printf("Agent %s OpenAI error: %v", agent.ID, err)
				continue
			}

			// Process the completion
			agent.processCompletion(ctx, chat)

		default:
			// Check for messages from other agents
			messages := messageBus.GetMessages(agent.ID)
			if len(messages) > 0 {
				log.Printf("Agent '%s' received %d messages", agent.ID, len(messages))
				// Process messages and update context
				for _, msg := range messages {
					agent.processMessage(msg)
				}
			}

			// Get completion from OpenAI for any pending messages
			chat, err := client.Chat.Completions.New(ctx, agent.Params)
			if err != nil {
				log.Printf("Agent %s OpenAI error: %v", agent.ID, err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Process the completion
			agent.processCompletion(ctx, chat)
		}

		// Sleep briefly to prevent tight loop
		time.Sleep(100 * time.Millisecond)
	}
}

func (agent *Agent) processMessage(msg Message) {
	// Store message in context
	agent.storeContext(fmt.Sprintf("message_from_%s", msg.From), msg.Content)

	// Add message to conversation
	messages := agent.Params.Messages.Value
	messages = append(messages, openai.UserMessage(
		fmt.Sprintf("Message from %s: %v", msg.From, msg.Content),
	))
	agent.Params.Messages.Value = messages
}

func (agent *Agent) processCompletion(ctx context.Context, chat *openai.ChatCompletion) {
	if chat == nil || len(chat.Choices) == 0 {
		return
	}

	choice := chat.Choices[0]
	if choice.Message.Content == "" {
		return
	}

	// Store completion in context
	agent.storeContext("last_completion", choice.Message.Content)

	// Add completion to conversation history
	messages := agent.Params.Messages.Value
	messages = append(messages, openai.AssistantMessage(choice.Message.Content))
	agent.Params.Messages.Value = messages

	// Log the agent's response
	log.Printf("Agent '%s' response: %s", agent.ID, choice.Message.Content)

	// Process any tool calls
	if len(choice.Message.ToolCalls) > 0 {
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Function.Name == "" {
				continue
			}

			log.Printf("Agent '%s' executing tool call: %s", agent.ID, toolCall.Function.Name)

			// Parse function arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				log.Printf("Agent %s tool call argument parse error: %v", agent.ID, err)
				continue
			}

			// Store tool call in context
			agent.storeContext(fmt.Sprintf("tool_call_%s", toolCall.Function.Name), args)

			// Handle tool call based on function name
			switch toolCall.Function.Name {
			case "send_agent_message":
				// Handle message sending
				if topic, ok := args["topic"].(string); ok {
					if content, ok := args["content"].(string); ok {
						msg := Message{
							From:    agent.ID,
							Topic:   topic,
							Content: content,
						}
						if err := messageBus.Publish(msg); err != nil {
							log.Printf("Agent '%s' failed to publish message: %v", agent.ID, err)
						} else {
							log.Printf("Agent '%s' published message to topic '%s'", agent.ID, topic)
						}
					}
				}
			case "system":
				// Handle system command
				if cmd, ok := args["command"].(string); ok {
					output, err := executeCommand(cmd, agent.Paths, agent.Tools)
					if err != nil {
						log.Printf("Agent '%s' system command error: %v", agent.ID, err)
						continue
					}
					log.Printf(
						"Agent '%s' executed system command '%s' with output: %s",
						agent.ID, cmd, output,
					)

					// Add command result to conversation
					messages := agent.Params.Messages.Value
					messages = append(messages, openai.AssistantMessage(
						fmt.Sprintf("Command executed: %s\nResult: %s", cmd, output),
					))
					agent.Params.Messages.Value = messages
				}
			}
		}
	}
}

func (agent *Agent) storeContext(key string, value interface{}) {
	agent.contextMutex.Lock()
	defer agent.contextMutex.Unlock()
	agent.contextStore[key] = value
}

func (agent *Agent) getContext(key string) (interface{}, bool) {
	agent.contextMutex.RLock()
	defer agent.contextMutex.RUnlock()
	value, exists := agent.contextStore[key]
	return value, exists
}

// executeCommand runs a command in a Docker container and returns its output
func executeCommand(cmd string, paths string, allowedTools string) (string, error) {
	// Parse allowed tools and paths
	tools := []string{}
	if allowedTools != "" {
		tools = strings.Split(allowedTools, ",")
		// Trim whitespace from each tool
		for i, tool := range tools {
			tools[i] = strings.TrimSpace(tool)
		}
	}

	// Get the main command (first word before any spaces)
	cmdParts := strings.Fields(cmd)
	if len(cmdParts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	mainCmd := cmdParts[0]

	// Check if the command is allowed
	if len(tools) > 0 && !slices.Contains(tools, mainCmd) {
		return "", fmt.Errorf("command '%s' is not allowed", mainCmd)
	}

	// Run the command in a container
	stdout, stderr, err := container.RunCommand(cmd, paths)
	if err != nil {
		return stderr, err
	}

	return stdout, nil
}
