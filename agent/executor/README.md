# Executor Agent

The Executor agent is responsible for executing individual steps from a plan using available tools. It is a critical component in the plan-execute-replan workflow.

## Overview

The Executor receives a plan with sequential steps and executes them one at a time using the available tools (file operations, code execution, project context). It generates syntactically correct code when needed and stores execution results in the ExecutionContext.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Executor Agent                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │         Chat Model (Tool Calling)                     │  │
│  │  - Analyzes step instruction                          │  │
│  │  - Decides which tools to use                         │  │
│  │  - Generates code when needed                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                  Tool System                          │  │
│  │  - read_file: Read project files                     │  │
│  │  - write_file: Write to project files                │  │
│  │  - list_directory: List directory contents           │  │
│  │  - execute_code: Run Python/JS/Go code               │  │
│  │  - get_project_context: Get project metadata         │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Execution Results                        │  │
│  │  - Step completion status                             │  │
│  │  - Generated code or output                           │  │
│  │  - Error messages if failed                           │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Creating an Executor

```go
import (
    "context"
    "github.com/cloudwego/eino/components/tool"
    "agent-server/agent/executor"
    "agent-server/agent/model"
    "agent-server/agent/tools"
)

func createExecutor(ctx context.Context, projectRoot string) (*executor.Executor, error) {
    // Create chat model
    chatModel, err := model.NewChatModel(ctx, &model.ChatModelConfig{
        Model: "gpt-4",
    })
    if err != nil {
        return nil, err
    }

    // Create tools
    toolsList := []tool.BaseTool{
        tools.NewReadFileTool(projectRoot),
        tools.NewWriteFileTool(projectRoot),
        tools.NewListDirectoryTool(projectRoot),
        tools.NewExecuteCodeTool(projectRoot),
        tools.NewGetProjectContextTool(projectRoot),
    }

    // Create executor
    exec, err := executor.NewExecutor(ctx, &executor.ExecutorConfig{
        Model:         chatModel,
        Tools:         toolsList,
        MaxIterations: 20,
    })
    if err != nil {
        return nil, err
    }

    return exec, nil
}
```

### Using with Plan-Execute Framework

The Executor is typically used as part of the plan-execute-replan workflow:

```go
import (
    "github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

func createPlanExecuteAgent(ctx context.Context) (adk.Agent, error) {
    planner, _ := createPlanner(ctx)
    executor, _ := createExecutor(ctx, "/path/to/project")
    replanner, _ := createReplanner(ctx)

    agent, err := planexecute.New(ctx, &planexecute.Config{
        Planner:       planner.Agent(),
        Executor:      executor.Agent(),
        Replanner:     replanner.Agent(),
        MaxIterations: 20,
    })
    
    return agent, err
}
```

## Configuration

### ExecutorConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `Model` | `model.ToolCallingChatModel` | Chat model for reasoning and tool calling | Required |
| `Tools` | `[]tool.BaseTool` | Available tools for execution | Required |
| `MaxIterations` | `int` | Maximum iterations per step execution | 20 |
| `Temperature` | `*float32` | Temperature for model generation | Model default |
| `MaxTokens` | `*int` | Maximum tokens in response | Model default |

## Execution Flow

1. **Receive Step**: Executor receives a step from the plan
2. **Analyze**: Chat model analyzes the step instruction
3. **Tool Selection**: Model decides which tools to use
4. **Execute**: Tools are invoked to accomplish the step
5. **Code Generation**: If needed, generates syntactically correct code
6. **Verification**: Verifies execution results
7. **Return Results**: Returns execution results to the framework

## Code Generation

When a step requires code generation, the Executor:

1. Analyzes the requirements from the step description
2. Generates syntactically correct code for the target language
3. Includes necessary imports and dependencies
4. Adds comments for clarity
5. Follows language-specific best practices
6. Uses the `write_file` tool to save the code
7. Optionally uses `execute_code` to verify the code works

### Example Code Generation

**Step**: "Create a Python script that reads a CSV file and calculates the average of the 'sales' column"

**Generated Code**:
```python
import pandas as pd

def calculate_average_sales(csv_path):
    """
    Read a CSV file and calculate the average of the 'sales' column.
    
    Args:
        csv_path: Path to the CSV file
        
    Returns:
        float: Average of the sales column
    """
    # Read the CSV file
    df = pd.read_csv(csv_path)
    
    # Calculate average of sales column
    average_sales = df['sales'].mean()
    
    return average_sales

if __name__ == "__main__":
    result = calculate_average_sales("data.csv")
    print(f"Average sales: {result}")
```

## Error Handling

The Executor handles errors gracefully:

- **Tool Errors**: Returns descriptive error messages from tools
- **Code Errors**: Captures syntax errors and runtime errors
- **Timeout**: Enforces max iterations limit
- **Invalid Steps**: Reports when a step cannot be executed

## Integration with ExecutionContext

The Executor stores results in the ExecutionContext:

```go
type ExecutionContext struct {
    UserInput     []*schema.Message
    Plan          planexecute.Plan
    ExecutedSteps []planexecute.ExecutedStep
}

type ExecutedStep struct {
    Step   string  // The step instruction
    Result string  // The execution result
}
```

## Best Practices

1. **Tool Selection**: Provide only the tools needed for your use case
2. **Max Iterations**: Set appropriate limits to prevent infinite loops
3. **Project Root**: Ensure project root is correctly set for file operations
4. **Error Handling**: Check execution results for errors
5. **Code Quality**: The executor generates production-quality code, not pseudocode

## Requirements Satisfied

This implementation satisfies the following requirements:

- **3.1**: Executes first step from Plan
- **3.2**: Uses available Tools to accomplish steps
- **3.3**: Generates syntactically correct code
- **3.5**: Stores execution results in ExecutionContext
- **3.6**: Supports configurable max iterations (default 20)
- **12.2**: Configurable max iterations parameter

## See Also

- [Planner Agent](../planner/README.md) - Creates execution plans
- [Replanner Agent](../replanner/README.md) - Evaluates and adjusts plans
- [Tool System](../tools/README.md) - Available tools for execution
- [Context Management](../context/README.md) - Session context management
