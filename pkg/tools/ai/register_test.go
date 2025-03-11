package ai

import (
	"testing"

	"github.com/openai/openai-go"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// TestRegisterAITools tests the RegisterAITools function
func TestRegisterAITools(t *testing.T) {
	Convey("Given the RegisterAITools function", t, func() {
		tools := RegisterAITools()

		Convey("It should return a non-empty slice of tools", func() {
			So(tools, ShouldNotBeNil)
			So(len(tools), ShouldBeGreaterThan, 0)
		})

		Convey("It should include all the agent tools", func() {
			// Count the tools by type
			var agentToolCount int
			var listAgentsToolCount int
			var sendCommandToolCount int
			var subscribeAgentToolCount int
			var killAgentToolCount int
			var sendAgentMessageToolCount int

			for _, tool := range tools {
				switch tool.(type) {
				case *AgentTool:
					agentToolCount++
				case *ListAgentsTool:
					listAgentsToolCount++
				case *SendCommandTool:
					sendCommandToolCount++
				case *SubscribeAgentTool:
					subscribeAgentToolCount++
				case *KillAgentTool:
					killAgentToolCount++
				case *SendAgentMessageTool:
					sendAgentMessageToolCount++
				}
			}

			// Verify each tool appears exactly once
			So(agentToolCount, ShouldEqual, 1)
			So(listAgentsToolCount, ShouldEqual, 1)
			So(sendCommandToolCount, ShouldEqual, 1)
			So(subscribeAgentToolCount, ShouldEqual, 1)
			So(killAgentToolCount, ShouldEqual, 1)
			So(sendAgentMessageToolCount, ShouldEqual, 1)
		})

		Convey("All registered tools should implement the core.Tool interface", func() {
			for _, tool := range tools {
				So(tool, ShouldImplement, (*core.Tool)(nil))
			}
		})
	})
}

// TestGetAllToolsAsOpenAI tests the GetAllToolsAsOpenAI function
func TestGetAllToolsAsOpenAI(t *testing.T) {
	Convey("Given the GetAllToolsAsOpenAI function", t, func() {
		openaiTools := GetAllToolsAsOpenAI()

		Convey("It should return a non-empty slice of OpenAI tools", func() {
			So(openaiTools, ShouldNotBeNil)
			So(len(openaiTools), ShouldBeGreaterThan, 0)
		})

		Convey("All tools should be in OpenAI function calling format", func() {
			for _, tool := range openaiTools {
				// Check that it's a function tool
				So(tool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)

				// Check that function parameters are defined
				So(tool.Function.Value.Name.Value, ShouldNotBeEmpty)
				So(tool.Function.Value.Parameters.Value, ShouldNotBeNil)

				// Check that required parameters field exists
				params := tool.Function.Value.Parameters.Value
				_, hasProperties := params["properties"]
				_, hasRequired := params["required"]

				So(hasProperties, ShouldBeTrue)
				So(hasRequired, ShouldBeTrue)
			}
		})

		Convey("Tool count should match between RegisterAITools and GetAllToolsAsOpenAI", func() {
			tools := RegisterAITools()
			So(len(openaiTools), ShouldEqual, len(tools))
		})

		Convey("Each agent tool should have a corresponding OpenAI tool", func() {
			// Get all the tool names from RegisterAITools
			toolNames := make([]string, 0)
			for _, tool := range RegisterAITools() {
				toolName := tool.Handle().Name
				toolNames = append(toolNames, toolName)
			}

			// Check that each OpenAI tool has a name that matches one of our registered tools
			for _, openaiTool := range openaiTools {
				name := openaiTool.Function.Value.Name.Value
				found := false
				for _, toolName := range toolNames {
					if name == toolName {
						found = true
						break
					}
				}
				So(found, ShouldBeTrue)
			}
		})
	})
}
