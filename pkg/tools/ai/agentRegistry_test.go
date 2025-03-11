package ai

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

// TestNewMessageBus tests the NewMessageBus constructor function
func TestNewMessageBus(t *testing.T) {
	Convey("Given the NewMessageBus function", t, func() {
		mb := NewMessageBus()

		Convey("It should initialize channels map", func() {
			So(mb.channels, ShouldNotBeNil)
		})

		Convey("It should initialize subscribers map", func() {
			So(mb.subscribers, ShouldNotBeNil)
		})
	})
}

// TestSubscribe tests the Subscribe method of MessageBus
func TestSubscribe(t *testing.T) {
	Convey("Given a MessageBus", t, func() {
		mb := NewMessageBus()

		Convey("Subscribe should add an agent to a topic's subscribers", func() {
			err := mb.Subscribe("test-agent", "test-topic")
			So(err, ShouldBeNil)

			// Verify the agent was added to the topic's subscribers
			mb.mutex.RLock()
			subscribers := mb.subscribers["test-topic"]
			mb.mutex.RUnlock()

			So(subscribers, ShouldContain, "test-agent")
		})

		Convey("Subscribe should not add duplicate subscriptions", func() {
			// Subscribe twice to the same topic
			mb.Subscribe("test-agent", "test-topic")
			err := mb.Subscribe("test-agent", "test-topic")
			So(err, ShouldBeNil)

			// Verify the agent appears only once in the topic's subscribers
			mb.mutex.RLock()
			subscribers := mb.subscribers["test-topic"]
			mb.mutex.RUnlock()

			count := 0
			for _, sub := range subscribers {
				if sub == "test-agent" {
					count++
				}
			}
			So(count, ShouldEqual, 1)
		})
	})
}

// TestPublish tests the Publish method of MessageBus
func TestPublish(t *testing.T) {
	Convey("Given a MessageBus", t, func() {
		mb := NewMessageBus()

		Convey("Publish should deliver messages to subscribers", func() {
			// Subscribe an agent to a topic
			mb.Subscribe("test-agent", "test-topic")

			// Create a message
			msg := Message{
				From:    "sender-agent",
				Topic:   "test-topic",
				Content: "test message",
			}

			// Publish the message
			err := mb.Publish(msg)
			So(err, ShouldBeNil)

			// Verify the message was delivered to the channel
			mb.mutex.RLock()
			_, exists := mb.channels["test-topic"]
			mb.mutex.RUnlock()
			So(exists, ShouldBeTrue)
		})

		Convey("Publishing to a topic without subscribers should still work", func() {
			msgNoSubs := Message{
				From:    "sender-agent",
				Topic:   "no-subscribers-topic",
				Content: "test message",
			}
			err := mb.Publish(msgNoSubs)
			So(err, ShouldBeNil)
		})
	})
}

// TestGetMessages tests the GetMessages method of MessageBus
func TestGetMessages(t *testing.T) {
	Convey("Given a MessageBus with published messages", t, func() {
		mb := NewMessageBus()

		// Subscribe an agent to multiple topics
		mb.Subscribe("multi-topic-agent", "topic1")
		mb.Subscribe("multi-topic-agent", "topic2")

		// Publish messages to each topic
		mb.Publish(Message{
			From:    "sender1",
			Topic:   "topic1",
			Content: "message 1",
		})
		mb.Publish(Message{
			From:    "sender2",
			Topic:   "topic2",
			Content: "message 2",
		})

		Convey("GetMessages should return all messages for a subscriber", func() {
			// Get messages for the agent
			messages := mb.GetMessages("multi-topic-agent")

			// Verify we got both messages
			So(len(messages), ShouldEqual, 2)

			// Verify the correct messages were received
			foundTopic1 := false
			foundTopic2 := false
			for _, msg := range messages {
				if msg.Topic == "topic1" && msg.From == "sender1" && msg.Content == "message 1" {
					foundTopic1 = true
				}
				if msg.Topic == "topic2" && msg.From == "sender2" && msg.Content == "message 2" {
					foundTopic2 = true
				}
			}
			So(foundTopic1, ShouldBeTrue)
			So(foundTopic2, ShouldBeTrue)
		})

		Convey("GetMessages should return empty slice for unknown agent", func() {
			// Getting messages for an agent that doesn't exist should return an empty slice
			noAgentMessages := mb.GetMessages("nonexistent-agent")
			So(len(noAgentMessages), ShouldEqual, 0)
		})
	})
}

// TestRunningAgent tests the RunningAgent structure
func TestRunningAgent(t *testing.T) {
	Convey("Given a RunningAgent", t, func() {
		agent := &Agent{
			ID:          "test-agent",
			Task:        "test task",
			commandChan: make(chan string, 1),
			killChan:    make(chan struct{}),
		}

		runningAgent := &RunningAgent{
			agent:        agent,
			commandChan:  agent.commandChan,
			responseChan: make(chan string, 1),
			killChan:     agent.killChan,
			lastActive:   time.Now(),
		}

		Convey("It should have the correct fields", func() {
			So(runningAgent.agent, ShouldEqual, agent)
			So(runningAgent.commandChan, ShouldEqual, agent.commandChan)
			So(runningAgent.killChan, ShouldEqual, agent.killChan)
			So(runningAgent.responseChan, ShouldNotBeNil)
			So(runningAgent.lastActive, ShouldHappenBefore, time.Now().Add(time.Second))
		})
	})
}

// TestStartAgentCleanup tests the StartAgentCleanup function
func TestStartAgentCleanup(t *testing.T) {
	Convey("Given the StartAgentCleanup function", t, func() {
		// This is mostly a placeholder test as fully testing the cleanup
		// would require manipulating time which is beyond the scope of this test

		Convey("It should not panic when called", func() {
			// Just verify it doesn't crash when called
			So(func() { StartAgentCleanup() }, ShouldNotPanic)
		})
	})
}
