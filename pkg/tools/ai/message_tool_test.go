package ai

import (
	"testing"

	"github.com/openai/openai-go"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// TestSendAgentMessageTool tests the SendAgentMessageTool structure and methods
func TestSendAgentMessageTool(t *testing.T) {
	Convey("Given a SendAgentMessageTool", t, func() {
		tool := NewSendAgentMessageTool()

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			handle := tool.Handle()
			So(handle.Name, ShouldEqual, "send_agent_message")
		})

		Convey("ToOpenAITool should return the correct OpenAI tool format", func() {
			openaiTool := tool.(*SendAgentMessageTool).ToOpenAITool()

			So(openaiTool.Type.Value, ShouldEqual, openai.ChatCompletionToolTypeFunction)
			So(openaiTool.Function.Value.Name.Value, ShouldEqual, "send_agent_message")

			// Check that parameters exist
			params := openaiTool.Function.Value.Parameters.Value
			properties, ok := params["properties"].(map[string]any)
			So(ok, ShouldBeTrue)

			// Verify specific properties exist
			_, hasTopic := properties["topic"]
			_, hasContent := properties["content"]

			So(hasTopic, ShouldBeTrue)
			So(hasContent, ShouldBeTrue)

			// Verify required fields
			required, ok := params["required"].([]string)
			So(ok, ShouldBeTrue)
			So(required, ShouldContain, "topic")
			So(required, ShouldContain, "content")
		})

		// For the Handler tests, we would typically need to provide a mock
		// implementation of mcp.CallToolRequest. Without knowing the exact structure,
		// we'll focus on testing the other methods.
	})
}
