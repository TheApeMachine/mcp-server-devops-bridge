package ai

import (
	"testing"

	"github.com/openai/openai-go"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// TestNewAgentTool tests the NewAgentTool constructor
func TestNewAgentTool(t *testing.T) {
	Convey("Given the NewAgentTool function", t, func() {
		tool := NewAgentTool()

		Convey("It should return a non-nil tool", func() {
			So(tool, ShouldNotBeNil)
		})

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			So(tool.Handle().Name, ShouldEqual, "agent")
		})
	})
}

// TestAgentToolHandle tests the Handle method of AgentTool
func TestAgentToolHandle(t *testing.T) {
	Convey("Given an AgentTool", t, func() {
		tool := NewAgentTool().(*AgentTool)

		Convey("Handle should return the correct mcp.Tool", func() {
			handle := tool.Handle()
			So(handle, ShouldNotBeNil)
			So(handle.Name, ShouldEqual, "agent")
		})
	})
}

// TestAgentToolToOpenAITool tests the ToOpenAITool method of AgentTool
func TestAgentToolToOpenAITool(t *testing.T) {
	Convey("Given an AgentTool", t, func() {
		tool := NewAgentTool().(*AgentTool)

		Convey("ToOpenAITool should return a properly formatted OpenAI tool", func() {
			openaiTool := tool.ToOpenAITool()

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
	})
}

// TestNewListAgentsTool tests the NewListAgentsTool constructor
func TestNewListAgentsTool(t *testing.T) {
	Convey("Given the NewListAgentsTool function", t, func() {
		tool := NewListAgentsTool()

		Convey("It should return a non-nil tool", func() {
			So(tool, ShouldNotBeNil)
		})

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			So(tool.Handle().Name, ShouldEqual, "list_agents")
		})
	})
}

// TestListAgentsToolHandle tests the Handle method of ListAgentsTool
func TestListAgentsToolHandle(t *testing.T) {
	Convey("Given a ListAgentsTool", t, func() {
		tool := NewListAgentsTool().(*ListAgentsTool)

		Convey("Handle should return the correct mcp.Tool", func() {
			handle := tool.Handle()
			So(handle, ShouldNotBeNil)
			So(handle.Name, ShouldEqual, "list_agents")
		})
	})
}

// TestListAgentsToolToOpenAITool tests the ToOpenAITool method of ListAgentsTool
func TestListAgentsToolToOpenAITool(t *testing.T) {
	Convey("Given a ListAgentsTool", t, func() {
		tool := NewListAgentsTool().(*ListAgentsTool)

		Convey("ToOpenAITool should return a properly formatted OpenAI tool", func() {
			openaiTool := tool.ToOpenAITool()

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
	})
}

// TestNewSendCommandTool tests the NewSendCommandTool constructor
func TestNewSendCommandTool(t *testing.T) {
	Convey("Given the NewSendCommandTool function", t, func() {
		tool := NewSendCommandTool()

		Convey("It should return a non-nil tool", func() {
			So(tool, ShouldNotBeNil)
		})

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			So(tool.Handle().Name, ShouldEqual, "send_command")
		})
	})
}

// TestSendCommandToolHandle tests the Handle method of SendCommandTool
func TestSendCommandToolHandle(t *testing.T) {
	Convey("Given a SendCommandTool", t, func() {
		tool := NewSendCommandTool().(*SendCommandTool)

		Convey("Handle should return the correct mcp.Tool", func() {
			handle := tool.Handle()
			So(handle, ShouldNotBeNil)
			So(handle.Name, ShouldEqual, "send_command")
		})
	})
}

// TestSendCommandToolToOpenAITool tests the ToOpenAITool method of SendCommandTool
func TestSendCommandToolToOpenAITool(t *testing.T) {
	Convey("Given a SendCommandTool", t, func() {
		tool := NewSendCommandTool().(*SendCommandTool)

		Convey("ToOpenAITool should return a properly formatted OpenAI tool", func() {
			openaiTool := tool.ToOpenAITool()

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
	})
}

// TestNewSubscribeAgentTool tests the NewSubscribeAgentTool constructor
func TestNewSubscribeAgentTool(t *testing.T) {
	Convey("Given the NewSubscribeAgentTool function", t, func() {
		tool := NewSubscribeAgentTool()

		Convey("It should return a non-nil tool", func() {
			So(tool, ShouldNotBeNil)
		})

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			So(tool.Handle().Name, ShouldEqual, "subscribe_agent")
		})
	})
}

// TestSubscribeAgentToolHandle tests the Handle method of SubscribeAgentTool
func TestSubscribeAgentToolHandle(t *testing.T) {
	Convey("Given a SubscribeAgentTool", t, func() {
		tool := NewSubscribeAgentTool().(*SubscribeAgentTool)

		Convey("Handle should return the correct mcp.Tool", func() {
			handle := tool.Handle()
			So(handle, ShouldNotBeNil)
			So(handle.Name, ShouldEqual, "subscribe_agent")
		})
	})
}

// TestSubscribeAgentToolToOpenAITool tests the ToOpenAITool method of SubscribeAgentTool
func TestSubscribeAgentToolToOpenAITool(t *testing.T) {
	Convey("Given a SubscribeAgentTool", t, func() {
		tool := NewSubscribeAgentTool().(*SubscribeAgentTool)

		Convey("ToOpenAITool should return a properly formatted OpenAI tool", func() {
			openaiTool := tool.ToOpenAITool()

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
	})
}

// TestNewKillAgentTool tests the NewKillAgentTool constructor
func TestNewKillAgentTool(t *testing.T) {
	Convey("Given the NewKillAgentTool function", t, func() {
		tool := NewKillAgentTool()

		Convey("It should return a non-nil tool", func() {
			So(tool, ShouldNotBeNil)
		})

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			So(tool.Handle().Name, ShouldEqual, "kill_agent")
		})
	})
}

// TestKillAgentToolHandle tests the Handle method of KillAgentTool
func TestKillAgentToolHandle(t *testing.T) {
	Convey("Given a KillAgentTool", t, func() {
		tool := NewKillAgentTool().(*KillAgentTool)

		Convey("Handle should return the correct mcp.Tool", func() {
			handle := tool.Handle()
			So(handle, ShouldNotBeNil)
			So(handle.Name, ShouldEqual, "kill_agent")
		})
	})
}

// TestKillAgentToolToOpenAITool tests the ToOpenAITool method of KillAgentTool
func TestKillAgentToolToOpenAITool(t *testing.T) {
	Convey("Given a KillAgentTool", t, func() {
		tool := NewKillAgentTool().(*KillAgentTool)

		Convey("ToOpenAITool should return a properly formatted OpenAI tool", func() {
			openaiTool := tool.ToOpenAITool()

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
	})
}

// TestNewSendAgentMessageTool tests the NewSendAgentMessageTool constructor
func TestNewSendAgentMessageTool(t *testing.T) {
	Convey("Given the NewSendAgentMessageTool function", t, func() {
		tool := NewSendAgentMessageTool()

		Convey("It should return a non-nil tool", func() {
			So(tool, ShouldNotBeNil)
		})

		Convey("It should implement the core.Tool interface", func() {
			So(tool, ShouldImplement, (*core.Tool)(nil))
		})

		Convey("It should have the correct name", func() {
			So(tool.Handle().Name, ShouldEqual, "send_agent_message")
		})
	})
}

// TestSendAgentMessageToolHandle tests the Handle method of SendAgentMessageTool
func TestSendAgentMessageToolHandle(t *testing.T) {
	Convey("Given a SendAgentMessageTool", t, func() {
		tool := NewSendAgentMessageTool().(*SendAgentMessageTool)

		Convey("Handle should return the correct mcp.Tool", func() {
			handle := tool.Handle()
			So(handle, ShouldNotBeNil)
			So(handle.Name, ShouldEqual, "send_agent_message")
		})
	})
}

// TestSendAgentMessageToolToOpenAITool tests the ToOpenAITool method of SendAgentMessageTool
func TestSendAgentMessageToolToOpenAITool(t *testing.T) {
	Convey("Given a SendAgentMessageTool", t, func() {
		tool := NewSendAgentMessageTool().(*SendAgentMessageTool)

		Convey("ToOpenAITool should return a properly formatted OpenAI tool", func() {
			openaiTool := tool.ToOpenAITool()

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
	})
}
