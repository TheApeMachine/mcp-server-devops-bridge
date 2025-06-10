package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/theapemachine/mcp-server-devops-bridge/core/container"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

// Status represents the status of an agent.
type Status string

const (
	// StatusInitializing is the status for an agent that is being created.
	StatusInitializing Status = "initializing"
	// StatusRunning is the status for an agent that is currently processing.
	StatusRunning Status = "running"
	// StatusWaiting is the status for an agent that is waiting for input.
	StatusWaiting Status = "waiting_for_input"
	// StatusCompleted is the status for an agent that has completed its work.
	StatusCompleted Status = "completed"
	// StatusFailed is the status for an agent that has failed.
	StatusFailed Status = "failed"
)

// Agent represents a sub-agent managed by the AgentManager.
type Agent struct {
	ID           string
	container    *container.Container
	Status       Status
	SystemPrompt string
	Messages     []openai.ChatCompletionMessageParamUnion
	Result       string // The latest result from the agent
	taskChan     chan string
	shutdownChan chan struct{}
}

// AgentManager manages the lifecycle of agents.
type AgentManager struct {
	agents    map[string]*Agent
	mu        sync.RWMutex
	openaiCli *openai.Client
}

var (
	manager *AgentManager
	initErr error
	once    sync.Once
)

// NewAgentManager creates and returns a new AgentManager.
func NewAgentManager() (*AgentManager, error) {
	once.Do(func() {
		// Initialize OpenAI Client
		if os.Getenv("OPENAI_API_KEY") == "" {
			initErr = fmt.Errorf("OPENAI_API_KEY environment variable not set")
			return
		}
		openaiCli := openai.NewClient()

		manager = &AgentManager{
			agents:    make(map[string]*Agent),
			openaiCli: &openaiCli,
		}
	})
	return manager, initErr
}

// LaunchAgent creates a new agent, starts its execution loop, and creates a docker container.
func (m *AgentManager) LaunchAgent(systemPrompt, userPrompt string) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()

	// Create a new container manager for the agent
	agentContainer, err := container.NewContainer("debian:stable-slim")
	if err != nil {
		return nil, fmt.Errorf("failed to create container manager: %w", err)
	}

	// Start the container and keep it running
	err = agentContainer.Run(ctx, []string{"tail", "-f", "/dev/null"}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Inject meta-instructions into the system prompt
	enhancedSystemPrompt := systemPrompt + `

You are an autonomous agent running in a sandboxed Debian Linux container. You operate in an iterative loop:
1. You analyze the user's request and your current state (you can use the current context as a scratchpad).
2. You decide which tool to use and call it.
3. You receive the result from the tool.
4. You analyze the result and repeat the process, deciding on the next action (again, use the current context to make notes for yourself).
Use your available tools sequentially to break down the task and accomplish the goal.
When the entire task is finished, use the 'complete_task' tool.`

	id := uuid.New().String()
	agent := &Agent{
		ID:           id,
		container:    agentContainer,
		Status:       StatusInitializing,
		SystemPrompt: enhancedSystemPrompt,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(userPrompt),
		},
		taskChan:     make(chan string),
		shutdownChan: make(chan struct{}),
	}

	m.agents[id] = agent
	go m.runAgent(agent)
	return agent, nil
}

func (m *AgentManager) runAgent(agent *Agent) {
	agent.Status = StatusRunning
	defer func() {
		// If the loop exits, ensure the agent's status is not left as 'running'.
		if agent.Status == StatusRunning {
			agent.Status = StatusWaiting
		}
	}()

	// Define the tools available to the agent
	tools := []openai.ChatCompletionToolParam{
		{
			Function: openai.FunctionDefinitionParam{
				Name:        "complete_task",
				Description: openai.String("Mark the current task as complete and stop execution."),
			},
		},
		{
			Function: openai.FunctionDefinitionParam{
				Name:        "set_status",
				Description: openai.String("Set your own status to 'waiting_for_input' and pause execution. Use this when you are blocked or waiting for another agent's input."),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]string{
							"type":        "string",
							"description": "The status to set. Must be 'waiting_for_input'.",
						},
					},
					"required": []string{"status"},
				},
			},
		},
		{
			Function: openai.FunctionDefinitionParam{
				Name:        "list_agents",
				Description: openai.String("List all other available agents in the system to communicate with."),
			},
		},
		{
			Function: openai.FunctionDefinitionParam{
				Name:        "broadcast_message",
				Description: openai.String("Send a message to all other active agents."),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]string{
							"type":        "string",
							"description": "The message to broadcast.",
						},
					},
					"required": []string{"message"},
				},
			},
		},
		{
			Function: openai.FunctionDefinitionParam{
				Name:        "execute_command",
				Description: openai.String("Execute a shell command in the container."),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]string{
							"type":        "string",
							"description": "The shell command to execute.",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Function: openai.FunctionDefinitionParam{
				Name:        "send_message",
				Description: openai.String("Send a message to another agent."),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"recipient_id": map[string]string{
							"type":        "string",
							"description": "The ID of the recipient agent.",
						},
						"message": map[string]string{
							"type":        "string",
							"description": "The message to send.",
						},
					},
					"required": []string{"recipient_id", "message"},
				},
			},
		},
	}

	for {
		// Check for shutdown signal before making API call
		select {
		case <-agent.shutdownChan:
			m.mu.Lock()
			// container is stopped in ShutdownAgent
			delete(m.agents, agent.ID)
			m.mu.Unlock()
			return
		default:
			// continue
		}

		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4o,
			Messages: append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(agent.SystemPrompt)}, agent.Messages...),
			Tools:    tools,
		}

		completion, err := m.openaiCli.Chat.Completions.New(context.Background(), params)
		if err != nil {
			agent.Result = fmt.Sprintf("Error from LLM: %v", err)
			agent.Messages = append(agent.Messages, openai.UserMessage(agent.Result))
			time.Sleep(1 * time.Second) // Avoid rapid-fire errors
			continue
		}

		responseMessage := completion.Choices[0].Message
		agent.Messages = append(agent.Messages, responseMessage.ToParam())
		agent.Result = responseMessage.Content // Store latest text response

		// If there are no tool calls, the agent might be responding or asking a question.
		// We'll wait for the next instruction.
		if len(responseMessage.ToolCalls) == 0 {
			return // Exit loop and wait for new instructions
		}

		// Handle Tool Calls
		for _, toolCall := range responseMessage.ToolCalls {
			var toolResultContent string
			var toolErr error

			switch toolCall.Function.Name {
			case "complete_task":
				agent.Status = StatusCompleted
				agent.Result = "Task completed successfully."
				toolResultContent = "Task marked as complete. Agent is shutting down."
				// Append this final tool message before exiting
				agent.Messages = append(agent.Messages, openai.ToolMessage(toolResultContent, toolCall.ID))
				return // Exit the run loop

			case "set_status":
				var args struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					toolErr = fmt.Errorf("failed to unmarshal arguments for set_status: %w", err)
				} else if Status(args.Status) != StatusWaiting {
					toolErr = fmt.Errorf("invalid status '%s'. Only 'waiting_for_input' is allowed", args.Status)
				} else {
					agent.Status = StatusWaiting
					toolResultContent = "Status set to 'waiting_for_input'. Pausing execution."
					agent.Messages = append(agent.Messages, openai.ToolMessage(toolResultContent, toolCall.ID))
					return // Exit the loop
				}

			case "list_agents":
				agents := m.ListAgents()
				// Filter out the current agent from the list
				otherAgents := make([]map[string]string, 0)
				for _, a := range agents {
					if a.ID != agent.ID {
						otherAgents = append(otherAgents, map[string]string{
							"id":     a.ID,
							"status": string(a.Status),
							"result": a.Result,
						})
					}
				}

				if len(otherAgents) == 0 {
					toolResultContent = "No other agents are currently active."
				} else {
					jsonResult, err := json.Marshal(otherAgents)
					if err != nil {
						toolErr = fmt.Errorf("failed to serialize agent list: %w", err)
					} else {
						toolResultContent = string(jsonResult)
					}
				}

			case "broadcast_message":
				var args struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					toolErr = fmt.Errorf("failed to unmarshal arguments for broadcast_message: %w", err)
				} else {
					m.BroadcastMessage(agent.ID, args.Message)
					toolResultContent = "Message broadcasted to all other agents."
				}

			case "execute_command":
				var args struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					toolErr = fmt.Errorf("failed to unmarshal arguments for execute_command: %w", err)
				} else {
					toolResultContent, toolErr = m.executeInContainer(agent.ID, args.Command)
				}

			case "send_message":
				var args struct {
					RecipientID string `json:"recipient_id"`
					Message     string `json:"message"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					toolErr = fmt.Errorf("failed to unmarshal arguments for send_message: %w", err)
				} else {
					toolErr = m.SendMessageToAgent(agent.ID, args.RecipientID, args.Message)
					if toolErr == nil {
						toolResultContent = "Message sent successfully."
					}
				}

			default:
				toolErr = fmt.Errorf("unknown tool call: %s", toolCall.Function.Name)
			}

			if toolErr != nil {
				toolResultContent = fmt.Sprintf("Error: %v", toolErr)
			}

			agent.Messages = append(agent.Messages, openai.ToolMessage(toolResultContent, toolCall.ID))
		}
		// After processing tool calls, loop again to let the model process the results.
	}
}

// executeInContainer runs a command in the agent's dedicated docker container.
func (m *AgentManager) executeInContainer(agentID string, command string) (string, error) {
	m.mu.RLock()
	agent, exists := m.agents[agentID]
	if !exists {
		m.mu.RUnlock()
		return "", fmt.Errorf("agent not found")
	}
	m.mu.RUnlock()

	// Wrap the command in `sh -c` to correctly handle shell operators like '>'
	return agent.container.Execute(context.Background(), []string{"sh", "-c", command})
}

// GetAgentStatus retrieves the status of an agent.
func (m *AgentManager) GetAgentStatus(id string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent with ID %s not found", id)
	}

	return agent, nil
}

// InstructAgent sends a new instruction to a waiting agent.
func (m *AgentManager) InstructAgent(id string, prompt string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return fmt.Errorf("agent with ID %s not found", id)
	}

	if agent.Status != StatusWaiting {
		return fmt.Errorf("agent %s is not waiting for input, current status: %s", id, agent.Status)
	}

	// Append the new instruction and restart the agent's processing loop.
	agent.Messages = append(agent.Messages, openai.UserMessage(prompt))
	go m.runAgent(agent)
	return nil
}

// ShutdownAgent stops a running agent and its container.
func (m *AgentManager) ShutdownAgent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[id]
	if !exists {
		return fmt.Errorf("agent with ID %s not found", id)
	}

	// Stop and remove the container
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := agent.container.StopAndRemove(ctx); err != nil {
		// Log or handle the error if necessary, but don't block shutdown
	}

	// Signal the agent's goroutine to stop and remove from map
	close(agent.shutdownChan)
	delete(m.agents, id)

	return nil
}

// ListAgents returns a list of all active agents.
func (m *AgentManager) ListAgents() []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agentList := make([]*Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		agentList = append(agentList, agent)
	}
	return agentList
}

// BroadcastMessage sends a message to all other agents.
func (m *AgentManager) BroadcastMessage(senderID, message string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	formattedMessage := fmt.Sprintf("[Broadcast from Agent %s]: %s", senderID, message)

	for _, recipient := range m.agents {
		if recipient.ID != senderID {
			// Only wake up agents that are waiting for input
			if recipient.Status == StatusWaiting {
				recipient.Messages = append(recipient.Messages, openai.UserMessage(formattedMessage))
				go m.runAgent(recipient)
			}
		}
	}
}

// SendMessageToAgent allows one agent to send a message to another.
func (m *AgentManager) SendMessageToAgent(senderID, recipientID, message string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recipient, exists := m.agents[recipientID]
	if !exists {
		return fmt.Errorf("recipient agent with ID %s not found", recipientID)
	}

	if recipient.Status != StatusWaiting {
		return fmt.Errorf("recipient agent %s is not waiting for input", recipientID)
	}

	// Format the message to indicate the sender
	formattedMessage := fmt.Sprintf("[Message from Agent %s]: %s", senderID, message)

	// Send the message to the recipient by appending to its message log
	// and restarting its run loop.
	recipient.Messages = append(recipient.Messages, openai.UserMessage(formattedMessage))
	go m.runAgent(recipient)

	return nil
}
