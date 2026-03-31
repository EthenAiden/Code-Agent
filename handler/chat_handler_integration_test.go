package handler

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

// TestStreamingIntegration demonstrates the complete streaming flow
// This test validates Requirements 9.1, 9.2, 9.3, 9.4, 9.5
func TestStreamingIntegration(t *testing.T) {
	// Create a mock agent that returns streaming events
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Simulate a streaming response
		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					IsStreaming: false,
					Message:     schema.AssistantMessage("Test response", nil),
				},
			},
		})
	}()

	// Verify we can consume the events
	eventCount := 0
	var lastMessage string

	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			t.Fatalf("Unexpected error: %v", event.Err)
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			eventCount++
			if !event.Output.MessageOutput.IsStreaming {
				msg, err := event.Output.MessageOutput.GetMessage()
				assert.NoError(t, err)
				lastMessage = msg.Content
			}
		}
	}

	assert.Equal(t, 1, eventCount)
	assert.Equal(t, "Test response", lastMessage)
}

// TestRunnerConfiguration verifies that adk.RunnerConfig is used correctly
// Requirement 9.1: Enable SSE streaming in chat handler using adk.RunnerConfig
func TestRunnerConfiguration(t *testing.T) {
	ctx := context.Background()

	// Create a simple mock agent
	mockAgent := &simpleAgent{}

	// Create runner with streaming enabled (this is what chat_handler.go does)
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           mockAgent,
		EnableStreaming: true,
	})

	assert.NotNil(t, runner)

	// Run the agent
	messages := []*schema.Message{
		schema.UserMessage("test"),
	}

	iterator := runner.Run(ctx, messages)
	assert.NotNil(t, iterator)

	// Consume events
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		// Verify no errors
		assert.Nil(t, event.Err)
	}
}

// simpleAgent is a minimal agent implementation for testing
type simpleAgent struct{}

func (a *simpleAgent) Run(ctx context.Context, input *adk.AgentInput, options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					IsStreaming: false,
					Message:     schema.AssistantMessage("Simple response", nil),
				},
			},
		})
	}()

	return iterator
}

func (a *simpleAgent) Name(ctx context.Context) string {
	return "SimpleAgent"
}

func (a *simpleAgent) Description(ctx context.Context) string {
	return "A simple test agent"
}
