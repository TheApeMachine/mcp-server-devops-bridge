package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/openai/openai-go"
)

type Agent struct {
	ID           string
	SystemPrompt string
	Task         string
	Params       openai.ChatCompletionNewParams
	Paths        string
	Tools        string
	History      []openai.ChatCompletionMessageParamUnion
}

func SystemTool(cmds []string) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("system"),
			Description: openai.String("A linux bash command to run"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command to run",
						"enum":        cmds,
					},
				},
				"required": []string{"command"},
			}),
		}),
	}
}

// MessagingTool creates a function calling tool for agent-to-agent communication
func MessagingTool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: openai.F(openai.ChatCompletionToolTypeFunction),
		Function: openai.F(openai.FunctionDefinitionParam{
			Name:        openai.String("send_message"),
			Description: openai.String("Send a message to other agents via a topic"),
			Parameters: openai.F(openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"topic": map[string]interface{}{
						"type":        "string",
						"description": "The topic of the message",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content of the message",
					},
				},
				"required": []string{"topic", "content"},
			}),
		}),
	}
}

func NewAgent(id, systemPrompt, task, paths, tools string, cmds []string) *Agent {
	return &Agent{
		ID:           id,
		SystemPrompt: systemPrompt,
		Task:         task,
		Params: openai.ChatCompletionNewParams{
			Model:       openai.F(openai.ChatModelGPT4oMini),
			Tools:       openai.F([]openai.ChatCompletionToolParam{SystemTool(cmds), MessagingTool()}),
			Temperature: openai.F(0.0),
		},
		Paths:   paths,
		Tools:   tools,
		History: []openai.ChatCompletionMessageParamUnion{},
	}
}

func (a *Agent) Run() (string, error) {
	llm := openai.NewClient()
	ctx := context.Background()

	// If it's the first run, initialize the history
	if len(a.History) == 0 {
		a.History = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(a.SystemPrompt),
			openai.UserMessage(a.Task),
		}
	}

	// Use the full conversation history
	a.Params.Messages = openai.F(a.History)

	// Make the chat completion request
	chat, err := llm.Chat.Completions.New(ctx, a.Params)
	if err != nil {
		return "", err
	}

	// Add the assistant's response to the history
	a.History = append(a.History, chat.Choices[0].Message)

	// Check if there are tool calls in the response
	toolCalls := chat.Choices[0].Message.ToolCalls
	if len(toolCalls) == 0 {
		// No tool calls, return the message content
		return chat.Choices[0].Message.Content, nil
	}

	// Process each tool call
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "system" {
			// Extract the command from the function call arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return "", err
			}

			// Execute the command and get the output
			cmd := args["command"].(string)
			output, err := executeCommand(cmd, a.Paths, a.Tools)
			if err != nil {
				output = "Error executing command: " + err.Error()
			}

			// Add the tool response to the history
			a.History = append(a.History, openai.ToolMessage(toolCall.ID, output))
		} else if toolCall.Function.Name == "send_message" {
			// Extract message details
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return "", err
			}

			// Get message details
			topic, topicOk := args["topic"].(string)
			content, contentOk := args["content"].(string)

			if !topicOk || !contentOk {
				output := "Error: Invalid message format"
				a.History = append(a.History, openai.ToolMessage(toolCall.ID, output))
				continue
			}

			// Publish message to bus
			msg := Message{
				From:    a.ID,
				Topic:   topic,
				Content: content,
			}

			err := messageBus.Publish(msg)
			result := "Message sent successfully to topic: " + topic
			if err != nil {
				result = "Failed to send message: " + err.Error()
			}

			// Add the tool response to the history
			a.History = append(a.History, openai.ToolMessage(toolCall.ID, result))
		}
	}

	// Update params with full history
	a.Params.Messages = openai.F(a.History)

	// Make another chat completion request with the updated history
	chat, err = llm.Chat.Completions.New(ctx, a.Params)
	if err != nil {
		return "", err
	}

	// Add the final response to the history
	a.History = append(a.History, chat.Choices[0].Message)

	return chat.Choices[0].Message.Content, nil
}

// ProcessCommand handles a single command input, maintaining conversation history
func (a *Agent) ProcessCommand(command string) (string, error) {
	// Add user message to history
	a.History = append(a.History, openai.UserMessage(command))

	// Run the agent with the updated history
	return a.Run()
}

// StartAgentLoop runs the agent in a separate goroutine
func StartAgentLoop(agentID string, agent *Agent, commandChan chan string, responseChan chan string, killChan chan struct{}, ctx context.Context) {
	// Main agent loop
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, exit the loop
			return

		case <-killChan:
			// Kill signal received, exit the loop
			return

		case command := <-commandChan:
			// Update last active time in the registry
			agentsMutex.Lock()
			if runningAgent, exists := runningAgents[agentID]; exists {
				runningAgent.lastActive = time.Now()
			}
			agentsMutex.Unlock()

			// Process new command
			response, err := agent.ProcessCommand(command)
			if err != nil {
				responseChan <- "Error: " + err.Error()
				continue
			}

			// Check for messages from other agents
			messages := messageBus.GetMessages(agentID)
			if len(messages) > 0 {
				messageText := "Messages from other agents:\n"
				for _, msg := range messages {
					messageText += fmt.Sprintf("From %s on topic %s: %v\n",
						msg.From, msg.Topic, msg.Content)
				}

				// Add messages as system message and process
				agent.History = append(agent.History, openai.SystemMessage(messageText))

				followUpResponse, err := agent.Run()
				if err != nil {
					response += "\n\nAdditional messages received but error processing: " + err.Error()
				} else {
					response = followUpResponse
				}
			}

			// Send response back
			responseChan <- response
		}
	}
}

// executeCommand runs a command in a Docker container and returns its output
func executeCommand(cmd string, paths string, allowedTools string) (string, error) {
	// Split the command string into command and arguments
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Check if the command is in the list of allowed tools
	if !isCommandAllowed(parts[0], allowedTools) {
		return "", fmt.Errorf("command not allowed: %s", parts[0])
	}

	// Initialize Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %v", err)
	}
	cli.NegotiateAPIVersion(ctx)

	// Parse paths to create bind mounts
	bindMounts, err := createBindMounts(paths)
	if err != nil {
		return "", fmt.Errorf("failed to create bind mounts: %v", err)
	}

	// Create container config
	containerConfig := &container.Config{
		Image:        "alpine:latest", // Use lightweight Alpine Linux
		Cmd:          parts,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	// Create host config with bind mounts
	hostConfig := &container.HostConfig{
		Binds: bindMounts,
	}

	// Create the container
	resp, err := cli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %v", err)
	}

	// Add cleanup
	defer func() {
		cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
	}()

	// Start the container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %v", err)
	}

	// Wait for the container to finish
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("error waiting for container: %v", err)
		}
	case <-statusCh:
	}

	// Get container logs
	out, err := cli.ContainerLogs(
		ctx,
		resp.ID,
		container.LogsOptions{ShowStdout: true, ShowStderr: true},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %v", err)
	}
	defer out.Close()

	// Read the logs
	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, out)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %v", err)
	}

	return buf.String(), nil
}

// isCommandAllowed checks if a command is in the list of allowed tools
func isCommandAllowed(cmd string, allowedTools string) bool {
	if allowedTools == "" {
		return false
	}

	tools := strings.Split(allowedTools, ",")
	for _, tool := range tools {
		if strings.TrimSpace(tool) == cmd {
			return true
		}
	}
	return false
}

// createBindMounts creates Docker bind mounts from a comma-separated list of paths
func createBindMounts(paths string) ([]string, error) {
	if paths == "" {
		return []string{}, nil
	}

	var binds []string
	for _, path := range strings.Split(paths, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// Create a bind mount with read-only access
		binds = append(binds, fmt.Sprintf("%s:%s:ro", path, path))
	}

	return binds, nil
}
