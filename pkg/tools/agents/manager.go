package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/theapemachine/mcp-server-devops-bridge/core/container"

	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

// --- BrowserManager ---

// BrowserManager manages Rod browser instances for agents.
type BrowserManager struct {
	browsers map[string]*rod.Browser
	mu       sync.Mutex
}

var (
	browserManager *BrowserManager
	browserOnce    sync.Once
)

// NewBrowserManager creates and returns a new BrowserManager.
func NewBrowserManager() *BrowserManager {
	browserOnce.Do(func() {
		browserManager = &BrowserManager{
			browsers: make(map[string]*rod.Browser),
		}
	})
	return browserManager
}

// GetBrowserForAgent returns a browser instance for a given agent.
func (m *BrowserManager) GetBrowserForAgent(agentID string) (*rod.Browser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if browser, exists := m.browsers[agentID]; exists {
		return browser, nil
	}

	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	m.browsers[agentID] = browser
	return browser, nil
}

// CleanupForAgent closes the browser and cleans up resources for a given agent.
func (m *BrowserManager) CleanupForAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if browser, exists := m.browsers[agentID]; exists {
		_ = browser.Close() // Best-effort close
		delete(m.browsers, agentID)
	}
}

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
	ID               string
	container        *container.Container
	Status           Status
	SystemPrompt     string
	Messages         []openai.ChatCompletionMessageParamUnion
	Result           string // The latest result from the agent
	Temperature      float64
	MaxIterations    int
	CurrentIteration int
	taskChan         chan string
	shutdownChan     chan struct{}
	pendingMessages  []openai.ChatCompletionMessageParamUnion
	pendingMu        sync.Mutex
}

// AgentManager manages the lifecycle of agents.
type AgentManager struct {
	agents         map[string]*Agent
	mu             sync.RWMutex
	openaiCli      *openai.Client
	browserManager *BrowserManager
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
			agents:         make(map[string]*Agent),
			openaiCli:      &openaiCli,
			browserManager: NewBrowserManager(),
		}
	})
	return manager, initErr
}

// LaunchAgent creates a new agent, starts its execution loop, and creates a docker container.
func (m *AgentManager) LaunchAgent(systemPrompt, userPrompt string, temperature float64, maxIterations int) (*Agent, error) {
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
	enhancedSystemPrompt := systemPrompt + fmt.Sprintf(`

You are an autonomous agent running in a sandboxed Debian Linux container. You operate in an iterative loop with a maximum of %d iterations.
1. You analyze the user's request and your current state (you can use the current context as a scratchpad).
2. You decide which tool to use and call it. You have access to a shell via 'execute_command' and a web browser via 'browse_web' for research.
3. You receive the result from the tool.
4. You analyze the result and repeat the process, deciding on the next action.
Use your available tools sequentially to break down the task and accomplish the goal.
When the entire task is finished, use the 'complete_task' tool. If you reach the iteration limit, you must use 'complete_task' and summarize your work.`, maxIterations)

	id := uuid.New().String()
	agent := &Agent{
		ID:               id,
		container:        agentContainer,
		Status:           StatusInitializing,
		SystemPrompt:     enhancedSystemPrompt,
		Messages:         []openai.ChatCompletionMessageParamUnion{openai.UserMessage(userPrompt)},
		Temperature:      temperature,
		MaxIterations:    maxIterations,
		CurrentIteration: 0,
		pendingMessages:  make([]openai.ChatCompletionMessageParamUnion, 0),
		taskChan:         make(chan string),
		shutdownChan:     make(chan struct{}),
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
				Name:        "browse_web",
				Description: openai.String("Navigate to a URL and return its text content. Useful for research."),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]string{
							"type":        "string",
							"description": "The URL to browse.",
						},
					},
					"required": []string{"url"},
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
		// At the start of a cycle, absorb any messages that have been queued.
		agent.pendingMu.Lock()
		if len(agent.pendingMessages) > 0 {
			agent.Messages = append(agent.Messages, agent.pendingMessages...)
			agent.pendingMessages = make([]openai.ChatCompletionMessageParamUnion, 0) // Clear the queue
		}
		agent.pendingMu.Unlock()

		agent.CurrentIteration++

		// Check for shutdown signal or iteration limit before making API call
		if agent.CurrentIteration > agent.MaxIterations {
			agent.Status = StatusFailed
			agent.Result = fmt.Sprintf("Task failed: Exceeded maximum of %d iterations.", agent.MaxIterations)
			agent.Messages = append(agent.Messages, openai.UserMessage(agent.Result))
			return
		}

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

		// Prepare messages for this iteration
		apiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(agent.Messages)+2)
		apiMessages = append(apiMessages, openai.SystemMessage(agent.SystemPrompt))
		apiMessages = append(apiMessages, agent.Messages...)
		// Add a final context-setting user message for the current iteration
		apiMessages = append(apiMessages, openai.UserMessage(
			fmt.Sprintf("You are now on iteration %d of %d. Analyze the situation and decide your next tool call.", agent.CurrentIteration, agent.MaxIterations),
		))

		params := openai.ChatCompletionNewParams{
			Model:       openai.ChatModelGPT4o,
			Messages:    apiMessages,
			Tools:       tools,
			Temperature: openai.Opt(agent.Temperature),
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
			// Check for more pending work before going to sleep.
			agent.pendingMu.Lock()
			hasPending := len(agent.pendingMessages) > 0
			agent.pendingMu.Unlock()

			if hasPending {
				continue // New messages arrived, start a new work cycle immediately.
			}

			agent.Status = StatusWaiting
			return // No more work, exit loop and wait for new instructions
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

					// Before actually pausing, check if new work has arrived.
					agent.pendingMu.Lock()
					hasPending := len(agent.pendingMessages) > 0
					agent.pendingMu.Unlock()
					if hasPending {
						// New work is waiting, so don't pause. Continue to the next cycle.
						agent.Status = StatusRunning // Set status back to running
						continue
					}
					return // No new work, so exit the loop.
				}

			case "browse_web":
				var args struct {
					URL string `json:"url"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					toolErr = fmt.Errorf("failed to unmarshal arguments for browse_web: %w", err)
				} else {
					toolResultContent, toolErr = m.browseWeb(agent.ID, args.URL)
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

// browseWeb uses rod to navigate to a url and extract the main content text.
func (m *AgentManager) browseWeb(agentID, url string) (text string, err error) {
	// Defer a recover function to catch any panics from the rod library.
	// This prevents the entire server from crashing on a navigation error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic browsing to %s: %v", url, r)
		}
	}()

	browser, err := m.browserManager.GetBrowserForAgent(agentID)
	if err != nil {
		return "", fmt.Errorf("could not get browser: %w", err)
	}

	page := browser.MustPage(url)
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return "", fmt.Errorf("failed to wait for page load: %w", err)
	}

	// This JS function attempts to find the main content of the page,
	// stripping out common noise like navs, footers, and scripts.
	js := `() => {
		Array.from(
			document.querySelectorAll(
				'nav, footer, header, aside, script, style, img, iframe, video, audio, svg, [aria-hidden="true"]'
			)
		).forEach(el => el.remove());

		let bestElement = null;
		let maxScore = -1;
		const candidates = document.querySelectorAll('main, article, div');

		for (const el of candidates) {
			if (!el.textContent) continue;
			let score = el.textContent.trim().length;
			if (el.tagName === 'ARTICLE') score *= 1.5;
			else if (el.tagName === 'MAIN') score *= 1.2;
			if (score > maxScore) {
				maxScore = score;
				bestElement = el;
			}
		}

		if (bestElement) return bestElement.innerText;
		return document.body.innerText; // Fallback
	}`

	result, err := page.Evaluate(rod.Eval(js))
	if err != nil {
		return "", fmt.Errorf("failed to execute JS to extract content: %w", err)
	}

	textContent := result.Value.Str()

	// Limit the content size to avoid overwhelming the context.
	if len(textContent) > 8000 {
		textContent = textContent[:8000] + "... (content truncated)"
	}

	return textContent, nil
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

	// Queue the new instruction.
	agent.pendingMu.Lock()
	agent.pendingMessages = append(agent.pendingMessages, openai.UserMessage(prompt))
	agent.pendingMu.Unlock()

	// If the agent was waiting for input, it means its run loop is not active.
	// Wake it up by starting a new run loop, which will process the pending message.
	if agent.Status == StatusWaiting {
		go m.runAgent(agent)
	}

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

	// Clean up the browser instance for the agent
	m.browserManager.CleanupForAgent(id)

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
			// Queue the message for every other agent.
			recipient.pendingMu.Lock()
			recipient.pendingMessages = append(recipient.pendingMessages, openai.UserMessage(formattedMessage))
			recipient.pendingMu.Unlock()

			// If the agent was waiting for input, wake it up to process the new message.
			if recipient.Status == StatusWaiting {
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

	// Format the message to indicate the sender and queue it.
	formattedMessage := fmt.Sprintf("[Message from Agent %s]: %s", senderID, message)
	recipient.pendingMu.Lock()
	recipient.pendingMessages = append(recipient.pendingMessages, openai.UserMessage(formattedMessage))
	recipient.pendingMu.Unlock()

	// If the recipient was waiting, wake it up to process the message.
	if recipient.Status == StatusWaiting {
		go m.runAgent(recipient)
	}

	return nil
}
