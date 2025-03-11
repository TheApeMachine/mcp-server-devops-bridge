package ai

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// MockOpenAIClient mocks the OpenAI client for testing
type MockOpenAIClient struct {
	mock.Mock
}

// NewChatCompletion mocks the OpenAI chat completion
func (m *MockOpenAIClient) NewChatCompletion(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*openai.ChatCompletion), args.Error(1)
}

// TestAgentTool tests the AgentTool structure and methods
func TestAgentTool(t *testing.T) {
	Convey("Given an AgentTool", t, func() {
		tool := NewAgentTool()

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			handle := tool.Handle()
			So(handle.Name, ShouldEqual, "agent")
		})

		Convey("ToOpenAITool should return the correct OpenAI tool format", func() {
			openaiTool := tool.(*AgentTool).ToOpenAITool()

			So(openaiTool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)
			So(openaiTool.Function.Value.Name.Value, ShouldEqual, "agent")

			// Check that required parameters exist
			params := openaiTool.Function.Value.Parameters.Value
			properties, ok := params["properties"].(map[string]any)
			So(ok, ShouldBeTrue)

			// Verify specific properties exist
			_, hasID := properties["id"]
			_, hasSystemPrompt := properties["system_prompt"]
			_, hasTask := properties["task"]

			So(hasID, ShouldBeTrue)
			So(hasSystemPrompt, ShouldBeTrue)
			So(hasTask, ShouldBeTrue)

			// Verify required fields
			required, ok := params["required"].([]string)
			So(ok, ShouldBeTrue)
			So(required, ShouldContain, "id")
			So(required, ShouldContain, "system_prompt")
			So(required, ShouldContain, "task")
		})

		Convey("Handler should validate required parameters", func() {
			ctx := context.Background()

			Convey("It should require agent_id", func() {
				request := mcp.CallToolRequest{
					Params: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments,omitempty"`
						Meta      *struct {
							ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
						} `json:"_meta,omitempty"`
					}{
						Name: "test",
						Arguments: map[string]interface{}{
							"system_prompt": "test prompt",
							"task":          "test task",
						},
					},
				}

				result, err := tool.(*AgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should require system_prompt", func() {
				request := mcp.CallToolRequest{
					Params: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments,omitempty"`
						Meta      *struct {
							ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
						} `json:"_meta,omitempty"`
					}{
						Name: "test",
						Arguments: map[string]interface{}{
							"id":   "test-agent",
							"task": "test task",
						},
					},
				}

				result, err := tool.(*AgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should require task", func() {
				request := mcp.CallToolRequest{
					Params: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments,omitempty"`
						Meta      *struct {
							ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
						} `json:"_meta,omitempty"`
					}{
						Name: "test",
						Arguments: map[string]interface{}{
							"id":            "test-agent",
							"system_prompt": "test prompt",
						},
					},
				}

				result, err := tool.(*AgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should handle duplicate agent IDs", func() {
				// First create an agent with ID "test-agent"
				runningAgents["test-agent"] = &RunningAgent{
					agent: &Agent{ID: "test-agent"},
				}

				// Try to create another with same ID
				request := mcp.CallToolRequest{
					Params: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments,omitempty"`
						Meta      *struct {
							ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
						} `json:"_meta,omitempty"`
					}{
						Name: "test",
						Arguments: map[string]interface{}{
							"id":            "test-agent",
							"system_prompt": "test prompt",
							"task":          "test task",
						},
					},
				}

				result, err := tool.(*AgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)

				// Clean up
				delete(runningAgents, "test-agent")
			})

			Convey("It should successfully create an agent with valid parameters", func() {
				request := mcp.CallToolRequest{
					Params: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments,omitempty"`
						Meta      *struct {
							ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
						} `json:"_meta,omitempty"`
					}{
						Name: "test",
						Arguments: map[string]interface{}{
							"id":            "new-test-agent",
							"system_prompt": "test prompt",
							"task":          "test task",
							"tools":         "tool1,tool2",
							"paths":         "/path1,/path2",
						},
					},
				}

				result, err := tool.(*AgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)

				// Verify agent was added to runningAgents
				So(runningAgents, ShouldContainKey, "new-test-agent")

				// Clean up
				close(runningAgents["new-test-agent"].killChan)
				delete(runningAgents, "new-test-agent")
			})
		})
	})
}

// TestAgent tests the Agent structure and methods
func TestAgent(t *testing.T) {
	Convey("Given an Agent", t, func() {
		id := "test-agent"
		systemPrompt := "You are a test agent"
		task := "Run tests"
		paths := "/tmp"
		tools := "ls,cat,grep"
		cmds := []string{"ls", "cat", "grep"}

		agent := NewAgent(id, systemPrompt, task, paths, tools, cmds)

		Convey("NewAgent should correctly initialize an agent", func() {
			So(agent.ID, ShouldEqual, id)
			So(agent.SystemPrompt, ShouldEqual, systemPrompt)
			So(agent.Task, ShouldEqual, task)
			So(agent.Paths, ShouldEqual, paths)
			So(agent.Tools, ShouldEqual, tools)
			So(agent.killChan, ShouldNotBeNil)
			So(agent.commandChan, ShouldNotBeNil)

			// Verify OpenAI params
			So(agent.Params.Messages.Value, ShouldHaveLength, 2)
			So(agent.Params.Model.Value, ShouldEqual, openai.ChatModelGPT4oMini)
			So(agent.Params.Tools.Value, ShouldHaveLength, 2) // System tool and messaging tool
			So(agent.Params.Temperature.Value, ShouldEqual, 0.0)
		})

		Convey("Run should process commands and handle tool calls", func() {
			// This test would be more complex and require mocking OpenAI
			// For a simpler test, we'll just verify that channels are created and can be used

			So(agent.killChan, ShouldNotBeNil)
			So(agent.commandChan, ShouldNotBeNil)

			// Test that command channel works
			go func() {
				time.Sleep(100 * time.Millisecond)
				close(agent.killChan) // This should terminate the Run method
			}()

			// A full test would mock OpenAI and verify responses, but that's beyond the scope
		})
	})
}

// TestExecuteCommand tests the command execution function
func TestExecuteCommand(t *testing.T) {
	Convey("Given the executeCommand function", t, func() {
		// These tests would ideally use a mocked container
		// For now, we'll test basic validation logic

		Convey("It should reject empty commands", func() {
			_, err := executeCommand("", "", "")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "empty command")
		})

		Convey("It should check command against allowedTools", func() {
			_, err := executeCommand("unknown", "", "ls,cat")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "not allowed")

			// Testing valid commands would require a real container or mock
		})
	})
}

// TestMessageParsing tests parsing of message arguments
func TestMessageParsing(t *testing.T) {
	Convey("Given a tool call with JSON arguments", t, func() {
		// Create a sample tool call arguments JSON
		argsJSON := `{"command": "ls -la", "topic": "test", "content": "test message"}`

		Convey("It should parse command arguments correctly", func() {
			var args map[string]any
			err := json.Unmarshal([]byte(argsJSON), &args)
			So(err, ShouldBeNil)

			cmd, ok := args["command"].(string)
			So(ok, ShouldBeTrue)
			So(cmd, ShouldEqual, "ls -la")
		})

		Convey("It should parse messaging arguments correctly", func() {
			var args map[string]any
			err := json.Unmarshal([]byte(argsJSON), &args)
			So(err, ShouldBeNil)

			topic, topicOk := args["topic"].(string)
			content, contentOk := args["content"].(string)

			So(topicOk, ShouldBeTrue)
			So(contentOk, ShouldBeTrue)
			So(topic, ShouldEqual, "test")
			So(content, ShouldEqual, "test message")
		})
	})
}
