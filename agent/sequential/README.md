# Sequential Agent

The Sequential Agent is the top-level orchestrator for the Code-Agent architecture. It routes user requests to appropriate workflows based on intent classification.

## Overview

The Sequential Agent implements the following workflow:

1. **Intent Classification**: Classifies user queries into one of three types:
   - `chat`: Normal conversation
   - `generate_code`: Code generation requests
   - `modify_code`: Code modification requests

2. **Routing Logic**:
   - For `chat` intents: Bypasses plan-execute workflow and responds directly using the chat model
   - For `generate_code` and `modify_code` intents: Routes to the Plan-Execute agent

3. **Context Preservation**: Maintains ExecutionContext throughout the workflow

## Architecture

```
User Query
    ↓
Sequential Agent
    ↓
Intent Classifier
    ↓
    ├─→ [chat] → Chat Model → Direct Response
    │
    └─→ [generate_code/modify_code] → Plan-Execute Agent
                                           ↓
                                       Planner → Executor → Replanner
```

## Usage

### Creating a Sequential Agent

```go
import (
    "context"
    "github.com/ethen-aiden/code-agent/agent/sequential"
    "github.com/ethen-aiden/code-agent/agent/intent"
)

ctx := context.Background()

// Initialize components
intentClassifier := intent.NewIntentClassifier(chatModel)
planExecuteAgent, _ := planexecute.NewPlanExecuteAgent(ctx, config)

// Create sequential agent
sequentialAgent, err := sequential.NewSequentialAgent(ctx, &sequential.SequentialAgentConfig{
    IntentClassifier: intentClassifier,
    PlanExecuteAgent: planExecuteAgent,
    ChatModel:        chatModel,
    Name:             "CodeAgent",
    Description:      "AI coding assistant with intent-based routing",
})
```

### Running the Agent

```go
// Create input
input := &adk.AgentInput{
    Messages: []adk.Message{
        schema.UserMessage("Create a simple HTTP server in Go"),
    },
    EnableStreaming: true,
}

// Run agent
iterator := sequentialAgent.Run(ctx, input)

// Process events
for {
    event, ok := iterator.Next()
    if !ok {
        break
    }
    
    if event.Err != nil {
        // Handle error
        continue
    }
    
    if event.Output != nil && event.Output.MessageOutput != nil {
        // Handle output
        if event.Output.MessageOutput.IsStreaming {
            // Process streaming response
            stream := event.Output.MessageOutput.MessageStream
            // ...
        } else {
            // Process non-streaming response
            msg, _ := event.Output.MessageOutput.GetMessage()
            // ...
        }
    }
}
```

## Configuration

### SequentialAgentConfig

- `IntentClassifier`: Intent classifier for categorizing user queries
- `PlanExecuteAgent`: Plan-Execute agent for code generation/modification workflows
- `ChatModel`: Chat model for direct chat responses
- `Name`: Agent name (default: "SequentialAgent")
- `Description`: Agent description

## Requirements Satisfied

This implementation satisfies the following requirements:

- **5.1**: Sequential Agent invokes Intent Classifier first
- **5.2**: Routes requests to appropriate workflow based on intent
- **5.3**: Orchestrates Planner, Executor, and Replanner for code intents
- **5.4**: Maintains ExecutionContext throughout workflow
- **5.5**: Returns final result when workflow completes

## Implementation Details

### Intent-Based Routing

The Sequential Agent uses the Intent Classifier to determine the user's intent:

- **Chat Intent**: Queries that don't involve code work are handled directly by the chat model, bypassing the plan-execute workflow for faster responses.

- **Code Intents**: Queries requesting code generation or modification are routed to the Plan-Execute agent, which orchestrates the Planner, Executor, and Replanner.

### Streaming Support

The Sequential Agent supports both streaming and non-streaming responses:

- For chat intents with streaming enabled, the chat model's streaming API is used
- For code intents, streaming is handled by the Plan-Execute agent

### Error Handling

Errors are propagated through the iterator as `AgentEvent` objects with the `Err` field set. This allows the caller to handle errors gracefully without interrupting the workflow.

## Testing

Unit tests for the Sequential Agent can be found in `agent_test.go`. The tests cover:

- Intent classification and routing
- Chat intent handling
- Code intent handling
- Error handling
- Context preservation

## Integration

The Sequential Agent is integrated with the Chat Handler in `agent-server/handler/chat_handler.go`. The handler:

1. Loads message history from the database
2. Creates ExecutionContext with project ID and session parameters
3. Invokes the Sequential Agent with user messages
4. Streams responses via Server-Sent Events (SSE)
5. Stores agent responses in message history

## Future Enhancements

Potential future enhancements include:

- Support for additional intent types (e.g., "debug_code", "explain_code")
- Intent confidence thresholds for ambiguous queries
- Multi-turn conversation context management
- Human-in-the-loop framework selection
- Tool approval workflows
