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

func (tool *AgentTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	return &Agent{
		ID:           id,
		SystemPrompt: systemPrompt,
		Task:         task,
		Params: openai.ChatCompletionNewParams{
			Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(systemPrompt),
				openai.UserMessage(task),
			}),
			Model:       openai.F(openai.ChatModelGPT4oMini),
			Tools:       openai.F(openaiTools),
			Temperature: openai.F(0.0),
		},
		Paths:       paths,
		Tools:       tools,
		killChan:    make(chan struct{}),
		commandChan: make(chan string, 10),
	}
}

func (agent *Agent) Run() {
	llm := openai.NewClient()
	ctx := context.Background()

	log.Printf("Agent '%s' started with task: %s", agent.ID, agent.Task)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Agent '%s' terminated due to context cancellation", agent.ID)
			return
		case <-agent.killChan:
			log.Printf("Agent '%s' terminated by kill signal", agent.ID)
			return
		case command := <-agent.commandChan:
			log.Printf("Agent '%s' received command: %s", agent.ID, command)
			agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.UserMessage(command))
			chat, err := llm.Chat.Completions.New(ctx, agent.Params)

			if err != nil {
				log.Printf("Error running agent '%s': %v", agent.ID, err)
				return
			}

			// Add the assistant's response to the history
			agent.Params.Messages.Value = append(agent.Params.Messages.Value, chat.Choices[0].Message)

			// Check if there are tool calls in the response
			toolCalls := chat.Choices[0].Message.ToolCalls

			if len(toolCalls) == 0 {
				break
			}

			// Process each tool call
			for _, toolCall := range toolCalls {
				switch toolCall.Function.Name {
				case "system":
					var out strings.Builder
					// Extract the command from the function call arguments
					var args map[string]any

					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						out.WriteString(err.Error())
						agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, out.String()))
						continue
					}

					cmd := args["command"].(string)
					output, err := executeCommand(cmd, agent.Paths, agent.Tools)

					if err != nil {
						out.WriteString(err.Error())
						agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, out.String()))
						continue
					}

					agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, output))
				case "messaging":
					var (
						args map[string]any
						out  strings.Builder
					)

					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						out.WriteString(err.Error())
						agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, out.String()))
						continue
					}

					// Get message details
					topic, topicOk := args["topic"].(string)
					content, contentOk := args["content"].(string)

					if !topicOk || !contentOk {
						out.WriteString("Error: Invalid message format")
						agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, out.String()))
						continue
					}

					// Publish message to bus
					msg := Message{
						From:    agent.ID,
						Topic:   topic,
						Content: content,
					}

					err := messageBus.Publish(msg)
					result := "Message sent successfully to topic: " + topic

					if err != nil {
						out.WriteString("Failed to send message: " + err.Error())
						agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, out.String()))
						continue
					}

					out.WriteString(result)
					agent.Params.Messages.Value = append(agent.Params.Messages.Value, openai.ToolMessage(toolCall.ID, out.String()))
				}
			}
		default:
			// Check for messages addressed to this agent
			messages := messageBus.GetMessages(agent.ID)

			if len(messages) > 0 {
				for _, msg := range messages {
					content, ok := msg.Content.(string)
					if !ok {
						continue
					}

					msgText := fmt.Sprintf("Message from agent '%s' on topic '%s':\n%s",
						msg.From, msg.Topic, content)

					agent.commandChan <- msgText
				}
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
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
