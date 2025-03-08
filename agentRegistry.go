package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	// Registry of all running agents
	runningAgents = make(map[string]*RunningAgent)
	agentsMutex   sync.RWMutex

	// Message bus for inter-agent communication
	messageBus = NewMessageBus()
)

// RunningAgent represents an agent running in a goroutine
type RunningAgent struct {
	agent        *Agent
	commandChan  chan string        // Channel for sending new commands to the agent
	responseChan chan string        // Channel for receiving responses from the agent
	killChan     chan struct{}      // Channel for termination signal
	ctx          context.Context    // Context for the agent goroutine
	cancel       context.CancelFunc // Function to cancel the context
	lastActive   time.Time          // Timestamp of last activity for cleanup
}

// MessageBus handles communication between agents
type MessageBus struct {
	channels    map[string]chan Message // Map of topic channels
	subscribers map[string][]string     // Map of topics to subscriber agent IDs
	mutex       sync.RWMutex
}

// Message represents a message on the bus
type Message struct {
	From    string      // Sending agent ID
	Topic   string      // Message topic
	Content interface{} // Message content
}

// NewMessageBus creates a new message bus
func NewMessageBus() *MessageBus {
	return &MessageBus{
		channels:    make(map[string]chan Message),
		subscribers: make(map[string][]string),
	}
}

// Subscribe an agent to a topic
func (mb *MessageBus) Subscribe(agentID, topic string) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	// Create topic channel if it doesn't exist
	if _, exists := mb.channels[topic]; !exists {
		mb.channels[topic] = make(chan Message, 100) // Buffered channel
	}

	// Add agent to subscribers list
	mb.subscribers[topic] = append(mb.subscribers[topic], agentID)
	return nil
}

// Publish a message to a topic
func (mb *MessageBus) Publish(msg Message) error {
	mb.mutex.RLock()
	channel, exists := mb.channels[msg.Topic]
	mb.mutex.RUnlock()

	if !exists {
		return errors.New("topic does not exist")
	}

	// Non-blocking send
	select {
	case channel <- msg:
		return nil
	default:
		return errors.New("channel buffer full")
	}
}

// GetMessages retrieves messages for an agent from topics it's subscribed to
func (mb *MessageBus) GetMessages(agentID string) []Message {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	var messages []Message

	// Check all topics for this subscriber
	for topic, subscribers := range mb.subscribers {
		for _, subscriber := range subscribers {
			if subscriber == agentID {
				// Retrieve messages non-blocking
				channel := mb.channels[topic]

				// Drain up to 10 messages at a time
				for i := 0; i < 10; i++ {
					select {
					case msg := <-channel:
						messages = append(messages, msg)
					default:
						// No more messages
						break
					}
				}
			}
		}
	}

	return messages
}

// StartAgentCleanup runs in a goroutine to clean up idle agents
func StartAgentCleanup() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)

			var agentsToKill []string

			// Find agents idle for more than 1 hour
			agentsMutex.RLock()
			for id, agent := range runningAgents {
				if time.Since(agent.lastActive) > 1*time.Hour {
					agentsToKill = append(agentsToKill, id)
				}
			}
			agentsMutex.RUnlock()

			// Kill idle agents
			for _, id := range agentsToKill {
				agentsMutex.Lock()
				if agent, exists := runningAgents[id]; exists {
					close(agent.killChan)
					agent.cancel()
					delete(runningAgents, id)
					log.Printf("Auto-terminated idle agent: %s", id)
				}
				agentsMutex.Unlock()
			}
		}
	}()
}
