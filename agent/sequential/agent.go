/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sequential

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/ethen-aiden/code-agent/agent/intent"
)

// SequentialAgentConfig holds configuration for the Sequential Agent
type SequentialAgentConfig struct {
	// IntentClassifier classifies user queries into intent types
	IntentClassifier *intent.IntentClassifier

	// PlanExecuteAgent handles code generation and modification workflows
	PlanExecuteAgent adk.Agent

	// ChatModel is used for direct chat responses
	ChatModel model.ToolCallingChatModel

	// Name is the agent name
	Name string

	// Description is the agent description
	Description string
}

// SequentialAgent wraps the intent classification and routing logic
// It implements intent-based routing to either chat model or plan-execute agent
type SequentialAgent struct {
	intentClassifier *intent.IntentClassifier
	planExecuteAgent adk.Agent
	chatModel        model.ToolCallingChatModel
	name             string
	description      string
}

// NewSequentialAgent creates a new Sequential Agent that routes requests based on intent
// This is the top-level orchestrator that manages the flow between sub-agents
//
// The Sequential Agent performs the following workflow:
// 1. Classify user intent using IntentClassifier
// 2. For "chat" intents: bypass plan-execute and respond directly using ChatModel
// 3. For "generate_code" and "modify_code" intents: route to PlanExecuteAgent
// 4. Maintain ExecutionContext throughout the workflow
//
// Requirements: 5.1, 5.2, 5.3, 5.4, 5.5
func NewSequentialAgent(ctx context.Context, config *SequentialAgentConfig) (*SequentialAgent, error) {
	if config == nil {
		return nil, fmt.Errorf("sequential agent config cannot be nil")
	}

	if config.IntentClassifier == nil {
		return nil, fmt.Errorf("intent classifier cannot be nil")
	}

	if config.PlanExecuteAgent == nil {
		return nil, fmt.Errorf("plan-execute agent cannot be nil")
	}

	if config.ChatModel == nil {
		return nil, fmt.Errorf("chat model cannot be nil")
	}

	// Set default name and description if not provided
	name := config.Name
	if name == "" {
		name = "SequentialAgent"
	}

	description := config.Description
	if description == "" {
		description = "Sequential agent that routes requests based on intent classification"
	}

	return &SequentialAgent{
		intentClassifier: config.IntentClassifier,
		planExecuteAgent: config.PlanExecuteAgent,
		chatModel:        config.ChatModel,
		name:             name,
		description:      description,
	}, nil
}

// Run executes the sequential agent workflow with intent-based routing
// This method classifies the intent and routes to the appropriate handler
func (sa *SequentialAgent) Run(ctx context.Context, input *adk.AgentInput, options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	// Extract the user query from the last message
	messages := input.Messages
	if len(messages) == 0 {
		// Return an iterator with an error
		return sa.createErrorIterator(fmt.Errorf("no input messages provided"))
	}

	lastMessage := messages[len(messages)-1]
	userQuery := lastMessage.Content

	// Step 1: Classify the intent
	classification, err := sa.intentClassifier.Classify(ctx, userQuery)
	if err != nil {
		return sa.createErrorIterator(fmt.Errorf("failed to classify intent: %w", err))
	}

	// Step 2: Route based on intent
	switch classification.Intent {
	case intent.IntentChat:
		// For chat intents, bypass plan-execute and respond directly
		return sa.handleChatIntent(ctx, input)

	case intent.IntentGenerateCode, intent.IntentModifyCode:
		// For code intents, route to plan-execute agent
		return sa.planExecuteAgent.Run(ctx, input, options...)

	default:
		return sa.createErrorIterator(fmt.Errorf("unknown intent type: %s", classification.Intent))
	}
}

// handleChatIntent handles normal chat conversations by directly invoking the chat model
func (sa *SequentialAgent) handleChatIntent(ctx context.Context, input *adk.AgentInput) *adk.AsyncIterator[*adk.AgentEvent] {
	messages := input.Messages

	// Create an iterator and generator pair
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Check if streaming is enabled
		if input.EnableStreaming {
			// Generate a streaming response using the chat model
			stream, err := sa.chatModel.Stream(ctx, messages)
			if err != nil {
				generator.Send(&adk.AgentEvent{
					Err: fmt.Errorf("failed to generate streaming chat response: %w", err),
				})
				return
			}

			// Send the streaming response as an event
			generator.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming:   true,
						MessageStream: stream,
					},
				},
			})
		} else {
			// Generate a direct response using the chat model
			response, err := sa.chatModel.Generate(ctx, messages)
			if err != nil {
				generator.Send(&adk.AgentEvent{
					Err: fmt.Errorf("failed to generate chat response: %w", err),
				})
				return
			}

			// Send the response as an event
			generator.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming: false,
						Message:     response,
					},
				},
			})
		}
	}()

	return iterator
}

// createErrorIterator creates an iterator that immediately returns an error
func (sa *SequentialAgent) createErrorIterator(err error) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	generator.Send(&adk.AgentEvent{Err: err})
	generator.Close()
	return iterator
}

// GetPlanExecuteAgent returns the underlying plan-execute agent
// This is useful for testing and debugging
func (sa *SequentialAgent) GetPlanExecuteAgent() adk.Agent {
	return sa.planExecuteAgent
}

// GetChatModel returns the underlying chat model
// This is useful for testing and debugging
func (sa *SequentialAgent) GetChatModel() model.ToolCallingChatModel {
	return sa.chatModel
}

// GetIntentClassifier returns the underlying intent classifier
// This is useful for testing and debugging
func (sa *SequentialAgent) GetIntentClassifier() *intent.IntentClassifier {
	return sa.intentClassifier
}

// Name returns the agent name
// This method is required by the adk.Agent interface
func (sa *SequentialAgent) Name(ctx context.Context) string {
	return sa.name
}

// Description returns the agent description
// This method is required by the adk.Agent interface
func (sa *SequentialAgent) Description(ctx context.Context) string {
	return sa.description
}
