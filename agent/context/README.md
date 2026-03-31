# Context Manager

The Context Manager provides thread-safe, session-based parameter storage for the Code-Agent architecture. It enables maintaining state throughout multi-turn conversations and across the entire agent chain.

## Overview

The Context Manager uses Go's `sync.Map` for concurrent-safe storage of key-value pairs. It's designed to be embedded in Go's `context.Context` and propagated through the agent workflow:

```
Sequential Agent → Intent Classifier → Plan-Execute Agent → Planner → Executor → Replanner
```

## Features

- **Thread-Safe**: Uses `sync.Map` for concurrent access from multiple agents
- **Type-Safe Retrieval**: Generic `GetTypedContextParams[T]` function for type-safe parameter access
- **Session Isolation**: Each context maintains its own parameter storage
- **Flexible Storage**: Supports any Go type as values (strings, ints, structs, slices, maps, etc.)
- **Zero Dependencies**: Uses only Go standard library

## Usage

### Basic Usage

```go
import (
    "context"
    agentcontext "github.com/ethen-aiden/code-agent/agent/context"
)

// Initialize context with parameter storage
ctx := context.Background()
ctx = agentcontext.InitContextParams(ctx)

// Store parameters
agentcontext.AppendContextParams(ctx, map[string]interface{}{
    "project_id": "project-123",
    "user_id":    "user-456",
})

// Retrieve with type safety
projectID, ok := agentcontext.GetTypedContextParams[string](ctx, "project_id")
if ok {
    // Use projectID
}
```

### Common Context Keys

The following keys are commonly used throughout the agent system:

- `project_id` (string): Current project identifier
- `user_id` (string): Current user identifier
- `work_dir` (string): Working directory for file operations
- `project_structure` (map/struct): Cached project structure
- `execution_history` ([]ExecutedStep): Previous execution results
- `selected_framework` (string): User-selected frontend framework (Vue, React, React Native)
- `plan_id` (string): Current execution plan identifier

### Multi-Agent Workflow

```go
// Chat Handler: Initialize and store project context
ctx := agentcontext.InitContextParams(context.Background())
agentcontext.AppendContextParams(ctx, map[string]interface{}{
    "project_id": projectID,
    "work_dir":   workDir,
})

// Intent Classifier: Read context
projectID, _ := agentcontext.GetTypedContextParams[string](ctx, "project_id")

// Planner: Add plan information
agentcontext.AppendContextParams(ctx, map[string]interface{}{
    "plan_id": planID,
    "selected_framework": "React",
})

// Executor: Read all context
workDir, _ := agentcontext.GetTypedContextParams[string](ctx, "work_dir")
framework, _ := agentcontext.GetTypedContextParams[string](ctx, "selected_framework")
```

### Complex Data Types

The Context Manager supports storing complex data structures:

```go
type ProjectMetadata struct {
    Name         string
    Version      string
    Dependencies []string
    Config       map[string]interface{}
}

metadata := ProjectMetadata{
    Name:         "my-project",
    Version:      "1.0.0",
    Dependencies: []string{"dep1", "dep2"},
    Config: map[string]interface{}{
        "debug": true,
    },
}

agentcontext.AppendContextParams(ctx, map[string]interface{}{
    "project_metadata": metadata,
})

// Retrieve
retrieved, ok := agentcontext.GetTypedContextParams[ProjectMetadata](ctx, "project_metadata")
```

## API Reference

### InitContextParams

```go
func InitContextParams(ctx context.Context) context.Context
```

Initializes a new context with an empty ContextManager. This should be called at the start of each agent workflow.

### AppendContextParams

```go
func AppendContextParams(ctx context.Context, values map[string]interface{})
```

Adds or updates multiple key-value pairs in the context. Thread-safe and can be called concurrently.

### GetTypedContextParams

```go
func GetTypedContextParams[T any](ctx context.Context, key string) (T, bool)
```

Retrieves a typed value from the context parameters. Returns the value and a boolean indicating success.

### GetContextParams

```go
func GetContextParams(ctx context.Context, key string) (interface{}, bool)
```

Retrieves an untyped value from the context parameters.

### DeleteContextParams

```go
func DeleteContextParams(ctx context.Context, key string)
```

Removes a key from the context parameters.

### GetAllContextParams

```go
func GetAllContextParams(ctx context.Context) map[string]interface{}
```

Returns a snapshot of all context parameters as a map. Useful for debugging or serialization.

## Thread Safety

All operations are thread-safe and can be called concurrently from multiple goroutines:

```go
var wg sync.WaitGroup

// Concurrent writes
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        agentcontext.AppendContextParams(ctx, map[string]interface{}{
            "key": id,
        })
    }(i)
}

// Concurrent reads
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        agentcontext.GetTypedContextParams[int](ctx, "key")
    }()
}

wg.Wait()
```

## Requirements Satisfied

This implementation satisfies the following requirements from the Code-Agent Architecture spec:

- **8.1**: Maintains an Execution_Context for each user session
- **8.2**: Stores user input, current Plan, and executed steps
- **8.3**: Accessible to all agents in the workflow
- **8.4**: Supports storing custom parameters as key-value pairs
- **8.5**: Creates or updates context when new requests are received

## Testing

Run the test suite:

```bash
go test -v ./agent/context/
```

Run examples:

```bash
go test -v -run Example ./agent/context/
```

## Performance Considerations

- `sync.Map` is optimized for scenarios with stable keys and concurrent reads
- `GetAllContextParams` creates a copy of all parameters, so use sparingly for large contexts
- Type assertions in `GetTypedContextParams` are fast but should be used with correct types
- Context isolation ensures no cross-contamination between sessions
