package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/openai/openai-go"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/tools/ai"
)

func main() {
	// Get all our agent-related tools
	tools := ai.GetAllToolsAsOpenAI()

	// Create an OpenAI client
	client := openai.NewClient()

	// Use OpenAI to coordinate agents
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`You are a coordinator of AI agents. 
You will help create and coordinate multiple agents to work on tasks.
You can create agents, send them commands, and facilitate communication between them.`),
		openai.UserMessage("Create two agents: one named 'researcher' that can search for information, and one named 'writer' that can summarize information. Have the researcher find information about climate change and send it to the writer to summarize."),
	}

	// Initial parameters
	params := openai.ChatCompletionNewParams{
		Model:    openai.F(openai.ChatModelGPT4o),
		Messages: openai.F(messages),
		Tools:    openai.F(tools),
	}

	// Let's simulate a conversation where we create and coordinate agents
	ctx := context.Background()

	// We'll make 5 API calls to simulate a conversation
	for i := 0; i < 5; i++ {
		// Make the API call
		chat, err := client.Chat.Completions.New(ctx, params)
		if err != nil {
			log.Fatalf("Error calling OpenAI: %v", err)
		}

		// Add the assistant's response to our messages
		params.Messages.Value = append(params.Messages.Value, chat.Choices[0].Message)

		// Check for tool calls
		if len(chat.Choices[0].Message.ToolCalls) > 0 {
			// Process each tool call
			for _, toolCall := range chat.Choices[0].Message.ToolCalls {
				fmt.Printf("Tool Call: %s\n", toolCall.Function.Name)
				fmt.Printf("Arguments: %s\n\n", toolCall.Function.Arguments)

				// Here in a real application we would execute the tool
				// and get a real result. For this example, we'll simulate responses.
				var toolResult string

				switch toolCall.Function.Name {
				case "agent":
					toolResult = "Agent created successfully"
				case "list_agents":
					toolResult = "Running Agents:\n- ID: researcher\n  Task: Research climate change\n  Last Active: 2023-01-01T00:00:00Z\n- ID: writer\n  Task: Summarize information\n  Last Active: 2023-01-01T00:00:00Z"
				case "send_command":
					toolResult = "Command sent successfully"
				case "send_agent_message":
					toolResult = "Message sent successfully"
				case "subscribe_agent":
					toolResult = "Agent subscribed to topic successfully"
				default:
					toolResult = "Tool executed successfully"
				}

				// Add the tool result to our messages
				params.Messages.Value = append(params.Messages.Value, openai.ToolMessage(toolCall.ID, toolResult))
			}
		} else {
			// If there's no tool call, print the response
			fmt.Printf("AI Response: %s\n\n", chat.Choices[0].Message.Content)
		}

		// Add a user message if needed
		if i == 2 {
			params.Messages.Value = append(
				params.Messages.Value,
				openai.UserMessage("Great! Now have the writer agent summarize the information it received."),
			)
		}

		// Sleep to avoid hitting rate limits
		time.Sleep(1 * time.Second)
	}
}
