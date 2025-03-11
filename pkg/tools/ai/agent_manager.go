package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// Agent management tools

// ListAgentsTool lists all running agents
type ListAgentsTool struct {
	handle mcp.Tool
}

func NewListAgentsTool() core.Tool {
	return &ListAgentsTool{
		handle: mcp.NewTool(
			"list_agents",
			mcp.WithDescription("List all running agents"),
		),
	}
}

func (tool *ListAgentsTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *ListAgentsTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("list_agents"),
			Description: openai.String("List all running agents"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"random_string": map[string]any{
						"type":        "string",
						"description": "Dummy parameter for no-parameter tools",
					},
				},
				"required": []string{"random_string"},
			}),
		}),
	}
}

func (tool *ListAgentsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentsMutex.RLock()
	defer agentsMutex.RUnlock()

	if len(runningAgents) == 0 {
		return mcp.NewToolResultText("No agents are currently running."), nil
	}

	var out strings.Builder
	out.WriteString("Running Agents:\n")

	for id, agent := range runningAgents {
		lastActive := agent.lastActive.Format(time.RFC3339)
		out.WriteString(fmt.Sprintf("- ID: %s\n  Task: %s\n  Last Active: %s\n",
			id, agent.agent.Task, lastActive))
	}

	return mcp.NewToolResultText(out.String()), nil
}

// SendCommandTool sends a command to a running agent
type SendCommandTool struct {
	handle mcp.Tool
}

func NewSendCommandTool() core.Tool {
	return &SendCommandTool{
		handle: mcp.NewTool(
			"send_command",
			mcp.WithDescription("Send a command to a running agent"),
			mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to send command to")),
			mcp.WithString("command", mcp.Required(), mcp.Description("Command to send to the agent")),
		),
	}
}

func (tool *SendCommandTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *SendCommandTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("send_command"),
			Description: openai.String("Send a command to a running agent"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "ID of the agent to send command to",
					},
					"command": map[string]any{
						"type":        "string",
						"description": "Command to send to the agent",
					},
				},
				"required": []string{"agent_id", "command"},
			}),
		}),
	}
}

func (tool *SendCommandTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		agentID string
		command string
		ok      bool
	)

	if agentID, ok = request.Params.Arguments["agent_id"].(string); !ok {
		return mcp.NewToolResultError("Agent ID is required"), nil
	}

	if command, ok = request.Params.Arguments["command"].(string); !ok {
		return mcp.NewToolResultError("Command is required"), nil
	}

	agentsMutex.RLock()
	agent, exists := runningAgents[agentID]
	agentsMutex.RUnlock()

	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Agent with ID '%s' not found", agentID)), nil
	}

	// Update the last active timestamp
	agentsMutex.Lock()
	agent.lastActive = time.Now()
	agentsMutex.Unlock()

	// Send the command to the agent
	select {
	case agent.commandChan <- command:
		return mcp.NewToolResultText(fmt.Sprintf("Command sent to agent '%s'", agentID)), nil
	case <-time.After(5 * time.Second):
		return mcp.NewToolResultError(fmt.Sprintf("Timeout sending command to agent '%s'", agentID)), nil
	}
}

// SubscribeAgentTool subscribes an agent to a message topic
type SubscribeAgentTool struct {
	handle mcp.Tool
}

func NewSubscribeAgentTool() core.Tool {
	return &SubscribeAgentTool{
		handle: mcp.NewTool(
			"subscribe_agent",
			mcp.WithDescription("Subscribe an agent to a message topic"),
			mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
			mcp.WithString("topic", mcp.Required(), mcp.Description("Topic to subscribe to")),
		),
	}
}

func (tool *SubscribeAgentTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *SubscribeAgentTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("subscribe_agent"),
			Description: openai.String("Subscribe an agent to a message topic"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "ID of the agent",
					},
					"topic": map[string]any{
						"type":        "string",
						"description": "Topic to subscribe to",
					},
				},
				"required": []string{"agent_id", "topic"},
			}),
		}),
	}
}

func (tool *SubscribeAgentTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		agentID string
		topic   string
		ok      bool
	)

	if agentID, ok = request.Params.Arguments["agent_id"].(string); !ok {
		return mcp.NewToolResultError("Agent ID is required"), nil
	}

	if topic, ok = request.Params.Arguments["topic"].(string); !ok {
		return mcp.NewToolResultError("Topic is required"), nil
	}

	// Check if the agent exists
	agentsMutex.RLock()
	_, exists := runningAgents[agentID]
	agentsMutex.RUnlock()

	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Agent with ID '%s' not found", agentID)), nil
	}

	// Subscribe the agent to the topic
	err := messageBus.Subscribe(agentID, topic)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to subscribe agent to topic: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Agent '%s' subscribed to topic '%s'", agentID, topic)), nil
}

// KillAgentTool terminates a running agent
type KillAgentTool struct {
	handle mcp.Tool
}

func NewKillAgentTool() core.Tool {
	return &KillAgentTool{
		handle: mcp.NewTool(
			"kill_agent",
			mcp.WithDescription("Terminate a running agent"),
			mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to terminate")),
		),
	}
}

func (tool *KillAgentTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *KillAgentTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("kill_agent"),
			Description: openai.String("Terminate a running agent"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "ID of the agent to terminate",
					},
				},
				"required": []string{"agent_id"},
			}),
		}),
	}
}

func (tool *KillAgentTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		agentID string
		ok      bool
	)

	if agentID, ok = request.Params.Arguments["agent_id"].(string); !ok {
		return mcp.NewToolResultError("Agent ID is required"), nil
	}

	agentsMutex.RLock()
	agent, exists := runningAgents[agentID]
	agentsMutex.RUnlock()

	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Agent with ID '%s' not found", agentID)), nil
	}

	// Send kill signal to the agent
	close(agent.killChan)

	// Remove the agent from the registry
	agentsMutex.Lock()
	delete(runningAgents, agentID)
	agentsMutex.Unlock()

	return mcp.NewToolResultText(fmt.Sprintf("Agent '%s' terminated", agentID)), nil
}
