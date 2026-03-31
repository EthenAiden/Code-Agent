# Chat Model Integration

This package provides the chat model integration layer for the Code-Agent system. It wraps OpenAI-compatible APIs for use by all agents in the multi-agent architecture.

## Overview

The chat model is the underlying LLM used by all agents (Intent Classifier, Planner, Executor, Replanner) for reasoning and generation. It supports:

- **OpenAI-compatible APIs**: Works with OpenAI, Azure OpenAI, and other compatible providers
- **Streaming responses**: Real-time feedback through Server-Sent Events (SSE)
- **Tool calling**: Enables agents to invoke tools during execution
- **Flexible configuration**: Environment variables, config structs, or functional options

## Configuration

### Environment Variables (Primary)

The chat model loads configuration from environment variables first:

```bash
OPENAI_API_KEY=your-api-key-here
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4
```

### Configuration Struct (Fallback)

If environment variables are not set, configuration can be provided via struct:

```go
config := &model.ChatModelConfig{
    APIKey:  "your-api-key",
    Model:   "gpt-4",
    BaseURL: "https://api.openai.com/v1",
}

cm, err := model.NewChatModel(ctx, config)
```

### Functional Options (Alternative)

For more flexibility, use functional options:

```go
cm, err := model.NewChatModelWithOptions(ctx,
    model.WithAPIKey("your-api-key"),
    model.WithModel("gpt-4"),
    model.WithBaseURL("https://api.openai.com/v1"),
    model.WithTemperature(0.7),
    model.WithMaxTokens(2048),
    model.WithTopP(0.9),
)
```

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/ethen-aiden/code-agent/agent/model"
)

func main() {
    ctx := context.Background()
    
    // Create chat model (loads from environment variables)
    cm, err := model.NewChatModel(ctx, nil)
    if err != nil {
        log.Fatalf("failed to create chat model: %v", err)
    }
    
    // Use with agents
    agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:  "assistant",
        Model: cm,
    })
}
```

### Agent-Specific Configuration

Different agents may require different configurations:

```go
// Planner: Low temperature for deterministic planning
plannerModel, _ := model.NewChatModelWithOptions(ctx,
    model.WithTemperature(0.0),
    model.WithMaxTokens(4096),
)

// Executor: Moderate temperature for code generation
executorModel, _ := model.NewChatModelWithOptions(ctx,
    model.WithTemperature(0.5),
    model.WithMaxTokens(4096),
)

// Replanner: Higher temperature for creative re-planning
replannerModel, _ := model.NewChatModelWithOptions(ctx,
    model.WithTemperature(1.0),
    model.WithMaxTokens(4096),
)
```

## Requirements Satisfied

This implementation satisfies the following requirements from the spec:

- **6.1**: Support OpenAI-compatible Chat Model APIs ✓
- **6.2**: Configuration includes base URL, API key, and model name ✓
- **6.3**: Support streaming responses for real-time feedback ✓
- **6.4**: Support tool calling for agent-tool interactions ✓
- **6.5**: Configuration loadable from environment variables ✓
- **6.6**: Return descriptive error when Chat Model is unavailable ✓

## Error Handling

The chat model returns descriptive errors for common issues:

```go
cm, err := model.NewChatModel(ctx, nil)
if err != nil {
    // Possible errors:
    // - "OPENAI_API_KEY is required"
    // - "OPENAI_MODEL is required"
    // - "failed to create chat model: <underlying error>"
}
```

## Testing

Run tests with:

```bash
go test -v ./agent/model/...
```

Tests cover:
- Configuration loading from environment variables
- Configuration loading from struct
- Default values and fallbacks
- Error handling for missing required fields
- Functional options
- Environment variable precedence over config struct

## Integration

The chat model is integrated into the main application in `main.go`:

```go
func createChatModel(ctx context.Context) model.ToolCallingChatModel {
    cm, err := agentmodel.NewChatModel(ctx, nil)
    if err != nil {
        log.Fatalf("failed to create chatmodel: %v", err)
    }
    return cm
}
```

This ensures all agents use the same configuration source and can be easily reconfigured via environment variables.
