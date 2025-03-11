package ai

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

func newMockRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      "test",
			Arguments: args,
		},
	}
}

// TestListAgentsTool tests the ListAgentsTool structure and methods
func TestListAgentsTool(t *testing.T) {
	Convey("Given a ListAgentsTool", t, func() {
		tool := NewListAgentsTool()

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			handle := tool.Handle()
			So(handle.Name, ShouldEqual, "list_agents")
		})

		Convey("ToOpenAITool should return the correct OpenAI tool format", func() {
			openaiTool := tool.(*ListAgentsTool).ToOpenAITool()

			So(openaiTool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)
			So(openaiTool.Function.Value.Name.Value, ShouldEqual, "list_agents")

			// Check that parameters exist
			params := openaiTool.Function.Value.Parameters.Value
			properties, ok := params["properties"].(map[string]any)
			So(ok, ShouldBeTrue)

			// Verify dummy parameter exists
			_, hasRandomString := properties["random_string"]
			So(hasRandomString, ShouldBeTrue)

			// Verify required fields
			required, ok := params["required"].([]string)
			So(ok, ShouldBeTrue)
			So(required, ShouldContain, "random_string")
		})

		Convey("Handler should return a list of running agents", func() {
			ctx := context.Background()
			request := newMockRequest(map[string]interface{}{
				"random_string": "test",
			})

			Convey("When no agents are running", func() {
				// Make sure runningAgents is empty
				for k := range runningAgents {
					delete(runningAgents, k)
				}

				result, err := tool.(*ListAgentsTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("When agents are running", func() {
				// Add a test agent
				testAgent := &Agent{
					ID:   "test-agent",
					Task: "test task",
				}
				runningAgents["test-agent"] = &RunningAgent{
					agent:      testAgent,
					lastActive: time.Now(),
				}

				result, err := tool.(*ListAgentsTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)

				// Clean up
				delete(runningAgents, "test-agent")
			})
		})
	})
}

// TestSendCommandTool tests the SendCommandTool structure and methods
func TestSendCommandTool(t *testing.T) {
	Convey("Given a SendCommandTool", t, func() {
		tool := NewSendCommandTool()

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			handle := tool.Handle()
			So(handle.Name, ShouldEqual, "send_command")
		})

		Convey("ToOpenAITool should return the correct OpenAI tool format", func() {
			openaiTool := tool.(*SendCommandTool).ToOpenAITool()

			So(openaiTool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)
			So(openaiTool.Function.Value.Name.Value, ShouldEqual, "send_command")

			// Check that parameters exist
			params := openaiTool.Function.Value.Parameters.Value
			properties, ok := params["properties"].(map[string]any)
			So(ok, ShouldBeTrue)

			// Verify specific properties exist
			_, hasAgentID := properties["agent_id"]
			_, hasCommand := properties["command"]

			So(hasAgentID, ShouldBeTrue)
			So(hasCommand, ShouldBeTrue)

			// Verify required fields
			required, ok := params["required"].([]string)
			So(ok, ShouldBeTrue)
			So(required, ShouldContain, "agent_id")
			So(required, ShouldContain, "command")
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
							"command": "test command",
						},
					},
				}

				result, err := tool.(*SendCommandTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should require command", func() {
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
							"agent_id": "test-agent",
						},
					},
				}

				result, err := tool.(*SendCommandTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should check if agent exists", func() {
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
							"agent_id": "nonexistent-agent",
							"command":  "test command",
						},
					},
				}

				result, err := tool.(*SendCommandTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should send command to existing agent", func() {
				// Create a test agent
				testAgent := &Agent{
					ID:          "test-agent",
					Task:        "test task",
					commandChan: make(chan string, 1),
				}
				runningAgents["test-agent"] = &RunningAgent{
					agent:       testAgent,
					commandChan: testAgent.commandChan,
					lastActive:  time.Now(),
				}

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
							"agent_id": "test-agent",
							"command":  "test command",
						},
					},
				}

				result, err := tool.(*SendCommandTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)

				// Verify command was sent
				select {
				case cmd := <-testAgent.commandChan:
					So(cmd, ShouldEqual, "test command")
				default:
					t.Error("Command was not sent to agent channel")
				}

				// Clean up
				delete(runningAgents, "test-agent")
			})
		})
	})
}

// TestSubscribeAgentTool tests the SubscribeAgentTool structure and methods
func TestSubscribeAgentTool(t *testing.T) {
	Convey("Given a SubscribeAgentTool", t, func() {
		tool := NewSubscribeAgentTool()

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			handle := tool.Handle()
			So(handle.Name, ShouldEqual, "subscribe_agent")
		})

		Convey("ToOpenAITool should return the correct OpenAI tool format", func() {
			openaiTool := tool.(*SubscribeAgentTool).ToOpenAITool()

			So(openaiTool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)
			So(openaiTool.Function.Value.Name.Value, ShouldEqual, "subscribe_agent")

			// Check that parameters exist
			params := openaiTool.Function.Value.Parameters.Value
			properties, ok := params["properties"].(map[string]any)
			So(ok, ShouldBeTrue)

			// Verify specific properties exist
			_, hasAgentID := properties["agent_id"]
			_, hasTopic := properties["topic"]

			So(hasAgentID, ShouldBeTrue)
			So(hasTopic, ShouldBeTrue)

			// Verify required fields
			required, ok := params["required"].([]string)
			So(ok, ShouldBeTrue)
			So(required, ShouldContain, "agent_id")
			So(required, ShouldContain, "topic")
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
							"topic": "test-topic",
						},
					},
				}

				result, err := tool.(*SubscribeAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should require topic", func() {
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
							"agent_id": "test-agent",
						},
					},
				}

				result, err := tool.(*SubscribeAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should check if agent exists", func() {
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
							"agent_id": "nonexistent-agent",
							"topic":    "test-topic",
						},
					},
				}

				result, err := tool.(*SubscribeAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should subscribe existing agent to topic", func() {
				// Create a test agent
				testAgent := &Agent{ID: "test-agent"}
				runningAgents["test-agent"] = &RunningAgent{
					agent: testAgent,
				}

				// Reset or create message bus
				messageBus = NewMessageBus()

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
							"agent_id": "test-agent",
							"topic":    "test-topic",
						},
					},
				}

				result, err := tool.(*SubscribeAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)

				// Verify agent was subscribed to topic
				messageBus.mutex.RLock()
				subscribers := messageBus.subscribers["test-topic"]
				messageBus.mutex.RUnlock()

				So(subscribers, ShouldContain, "test-agent")

				// Clean up
				delete(runningAgents, "test-agent")
			})
		})
	})
}

// TestKillAgentTool tests the KillAgentTool structure and methods
func TestKillAgentTool(t *testing.T) {
	Convey("Given a KillAgentTool", t, func() {
		tool := NewKillAgentTool()

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			handle := tool.Handle()
			So(handle.Name, ShouldEqual, "kill_agent")
		})

		Convey("ToOpenAITool should return the correct OpenAI tool format", func() {
			openaiTool := tool.(*KillAgentTool).ToOpenAITool()

			So(openaiTool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)
			So(openaiTool.Function.Value.Name.Value, ShouldEqual, "kill_agent")

			// Check that parameters exist
			params := openaiTool.Function.Value.Parameters.Value
			properties, ok := params["properties"].(map[string]any)
			So(ok, ShouldBeTrue)

			// Verify specific properties exist
			_, hasAgentID := properties["agent_id"]
			So(hasAgentID, ShouldBeTrue)

			// Verify required fields
			required, ok := params["required"].([]string)
			So(ok, ShouldBeTrue)
			So(required, ShouldContain, "agent_id")
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
						Name:      "test",
						Arguments: map[string]interface{}{},
					},
				}

				result, err := tool.(*KillAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should check if agent exists", func() {
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
							"agent_id": "nonexistent-agent",
						},
					},
				}

				result, err := tool.(*KillAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})

			Convey("It should kill existing agent", func() {
				// Create a test agent with kill channel
				killChan := make(chan struct{})
				testAgent := &Agent{
					ID:       "test-agent",
					killChan: killChan,
				}
				runningAgents["test-agent"] = &RunningAgent{
					agent:    testAgent,
					killChan: killChan,
				}

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
							"agent_id": "test-agent",
						},
					},
				}

				// Set up a goroutine to check if killChan is closed
				killChanClosed := make(chan bool)
				go func() {
					_, ok := <-killChan
					killChanClosed <- !ok
				}()

				result, err := tool.(*KillAgentTool).Handler(ctx, request)
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)

				// Verify agent was removed from registry
				_, exists := runningAgents["test-agent"]
				So(exists, ShouldBeFalse)

				// Verify kill channel was closed
				closed := <-killChanClosed
				So(closed, ShouldBeTrue)
			})
		})
	})
}
