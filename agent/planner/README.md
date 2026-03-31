# Planner Agent

The Planner agent is responsible for creating execution plans from user requests in the Code-Agent architecture. It decomposes natural language requests into granular, sequential, and unambiguous steps that can be executed by the Executor agent.

## Overview

The Planner is a critical component in the plan-execute-replan workflow. It analyzes user requests, considers available context, and generates structured execution plans that guide the system toward accomplishing the user's goals.

## Components

### Plan Data Structure

The `Plan` struct represents a structured execution plan:

```go
type Plan struct {
    Steps []*Step  // List of steps to execute in order
    Goal  string   // Overall objective of the plan
}
```

Each `Step` contains:
- `ID`: Unique identifier for the step
- `Description`: Clear, actionable instruction
- `Executed`: Whether the step has been completed
- `Result`: Execution result (populated after execution)

### Planner Agent

The `Planner` struct implements the planning logic:

```go
type Planner struct {
    chatModel   model.ChatModel  // LLM for plan generation
    temperature *float32         // Randomness control
    maxTokens   *int            // Token limit
}
```

## Key Features

### 1. Plan Creation

The `CreatePlan` method generates a new execution plan:

```go
plan, err := planner.CreatePlan(ctx, userRequest, contextInfo)
```

**Process:**
1. Analyzes the user request
2. Considers available context (project info, file structure, etc.)
3. Generates a prompt for the LLM
4. Parses the LLM response into a structured Plan
5. Validates the plan structure

### 2. Plan Serialization

Plans support JSON serialization for storage and transmission:

```go
// Serialize
data, err := json.Marshal(plan)

// Deserialize
var plan Plan
err := json.Unmarshal(data, &plan)
```

### 3. Step Retrieval

The `FirstStep()` method retrieves the first unexecuted step:

```go
step := plan.FirstStep()
if step != nil {
    // Execute this step
}
```

### 4. Plan Updates

The `UpdatePlan` method modifies plans based on execution results:

```go
updatedPlan, err := planner.UpdatePlan(ctx, currentPlan, executionResults, contextInfo)
```

## Configuration

Create a Planner with custom configuration:

```go
config := &PlannerConfig{
    Model:       chatModel,
    Temperature: &temperature,  // Optional: 0.0-1.0
    MaxTokens:   &maxTokens,    // Optional: token limit
}

planner, err := NewPlanner(ctx, config)
```

## Planning Guidelines

The Planner follows these principles when creating plans:

1. **Granularity**: Each step is a single, clear action
2. **Sequential**: Steps are ordered logically
3. **Actionable**: Each step can be executed independently
4. **Specific**: Uses concrete language, avoids vagueness
5. **Context-Aware**: Considers project structure and available tools

## Available Tools

The Planner considers these tools when creating plans:

- `read_file`: Read content from project files
- `write_file`: Write content to project files
- `list_directory`: List directory contents
- `execute_code`: Execute code in Python, JavaScript, or Go
- `get_project_context`: Retrieve project metadata

## Example Usage

```go
// Create planner
config := &PlannerConfig{
    Model: chatModel,
}
planner, err := NewPlanner(ctx, config)
if err != nil {
    log.Fatal(err)
}

// Create plan
contextInfo := map[string]interface{}{
    "project_id": "123",
    "project_type": "web",
}

plan, err := planner.CreatePlan(
    ctx,
    "Create a REST API endpoint for user authentication",
    contextInfo,
)
if err != nil {
    log.Fatal(err)
}

// Use the plan
fmt.Println(plan.String())
for !plan.IsComplete() {
    step := plan.FirstStep()
    if step == nil {
        break
    }
    
    // Execute step...
    result := executeStep(step)
    
    // Mark as executed
    plan.MarkStepExecuted(step.ID, result)
}
```

## Plan Validation

The Planner validates plans to ensure:

- At least one step exists
- All steps have unique IDs
- All steps have non-empty descriptions
- Step IDs are positive integers
- No duplicate step IDs

## Integration

The Planner integrates with:

- **Chat Model**: Uses LLM for plan generation
- **Context Manager**: Accesses session context
- **Executor**: Provides plans for execution
- **Replanner**: Receives execution feedback for plan updates

## Requirements Satisfied

This implementation satisfies the following requirements:

- **2.1**: Creates plans when code generation/modification is detected
- **2.2**: Decomposes requests into granular, sequential steps
- **2.3**: Each step contains clear instructions
- **2.4**: Uses Chat Model for plan generation
- **2.5**: Plans are serializable to JSON
- **2.6**: Plan generation completes within 10 seconds
- **12.1**: Supports configurable temperature and max tokens

## Testing

See `planner_test.go` for unit tests covering:
- Plan creation with various requests
- Step decomposition quality
- Plan serialization/deserialization
- Configuration parameter handling
- Plan validation
- Error handling
