# Plan-Execute Agent

The Plan-Execute agent orchestrates the Planner, Executor, and Replanner agents in a loop to accomplish complex code generation tasks.

## Overview

This package implements the Plan-Execute-Replan pattern using CloudWeGo's Eino ADK framework. The agent:

1. **Plans**: Creates a step-by-step execution plan from user requests
2. **Executes**: Executes each step using available tools
3. **Replans**: Evaluates progress and adjusts the plan as needed

## Architecture

```
User Request
     ↓
  Planner → Creates execution plan
     ↓
  Executor → Executes first step
     ↓
  Replanner → Evaluates progress
     ↓
  Decision:
    - Finish: Task complete
    - Replan: Update plan
    - Continue: Next step
```

## Usage

```go
import (
    "context"
    "github.com/ethen-aiden/code-agent/agent/planexecute"
    "github.com/ethen-aiden/code-agent/agent/model"
    "github.com/ethen-aiden/code-agent/agent/tools"
)

func main() {
    ctx := context.Background()
    
    // Create chat model
    chatModel, err := model.NewChatModel(ctx, &model.ChatModelConfig{
        Temperature: ptrFloat32(0.7),
        MaxTokens:   ptrInt(4096),
    })
    if err != nil {
        panic(err)
    }
    
    // Create tools
    projectRoot := "/path/to/project"
    toolsList := []tool.BaseTool{
        tools.NewReadFileTool(projectRoot),
        tools.NewWriteFileTool(projectRoot),
        tools.NewListDirectoryTool(projectRoot),
        tools.NewExecuteCodeTool(projectRoot),
    }
    
    // Create plan-execute agent
    agent, err := planexecute.NewPlanExecuteAgent(ctx, &planexecute.PlanExecuteConfig{
        ChatModel:     chatModel,
        Tools:         toolsList,
        MaxIterations: 20,
    })
    if err != nil {
        panic(err)
    }
    
    // Use the agent...
}
```

## Configuration

### PlanExecuteConfig

- `ChatModel`: The LLM used by all agents (required)
- `Tools`: Available tools for the executor (required)
- `MaxIterations`: Maximum plan-execute-replan iterations (default: 20)
- `PlannerTemperature`: Temperature for plan generation (optional)
- `ExecutorTemperature`: Temperature for execution (optional)
- `ReplannerTemperature`: Temperature for replanning (optional)
- `MaxTokens`: Maximum tokens in responses (optional)

## Integration with Eino ADK

This agent uses the `planexecute.New` prebuilt from Eino ADK, which provides:

- Automatic iteration management
- Context propagation between agents
- Streaming support
- Error handling and recovery

## Requirements

Implements requirements:
- 3.6: Step execution with max iterations
- 4.1: Plan re-evaluation
- 5.3: Sequential agent orchestration
