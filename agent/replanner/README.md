# Replanner Agent

The Replanner agent is responsible for evaluating execution progress and deciding the next action in the plan-execute-replan workflow. It analyzes executed steps, reviews remaining steps, and determines whether to continue, replan, or finish.

## Overview

The Replanner is the decision-making component that closes the loop in the plan-execute-replan cycle. After the Executor completes a step, the Replanner evaluates the results and decides:

- **Continue**: Proceed with the remaining steps as planned
- **Replan**: Modify the plan based on execution results
- **Finish**: Task is complete, all goals achieved

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Replanner Agent                         │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Input:                                                       │
│  - User's original request                                    │
│  - Current plan with goal                                     │
│  - Executed steps and their results                           │
│  - Remaining unexecuted steps                                 │
│  - Project context                                            │
│                                                               │
│  Decision Process:                                            │
│  1. Check if all steps are complete                           │
│  2. Analyze execution results for errors                      │
│  3. Validate remaining steps are still appropriate            │
│  4. Make decision: finish, replan, or continue                │
│                                                               │
│  Output:                                                      │
│  - Decision action (finish/replan/continue)                   │
│  - Reasoning for the decision                                 │
│  - Updated plan (if replanning)                               │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Components

### ReplannerConfig

Configuration for creating a Replanner instance:

```go
type ReplannerConfig struct {
    // Model is the chat model used for replanning decisions
    Model model.ToolCallingChatModel
    
    // Temperature controls randomness (0.0 to 1.0)
    Temperature *float32
    
    // MaxTokens limits response size
    MaxTokens *int
    
    // RespondTool is the tool for submitting final results
    RespondTool *schema.ToolInfo
    
    // PlanTool is the tool for creating/updating plans
    PlanTool *schema.ToolInfo
}
```

### ReplannerDecision

Represents the decision made by the replanner:

```go
type ReplannerDecision struct {
    // Action: "continue", "replan", or "finish"
    Action string
    
    // Reasoning explains the decision
    Reasoning string
    
    // UpdatedPlan contains new plan if replanning
    UpdatedPlan *planner.Plan
}
```

## Usage

### Creating a Replanner

```go
import (
    "context"
    "agent-server/agent/model"
    "agent-server/agent/replanner"
)

func createReplanner(ctx context.Context) (*replanner.Replanner, error) {
    // Create chat model
    chatModel, err := model.NewChatModel(ctx, &model.ChatModelConfig{
        Temperature: ptrFloat32(1.0),
        MaxTokens:   ptrInt(4096),
    })
    if err != nil {
        return nil, err
    }
    
    // Create replanner
    replannerAgent, err := replanner.NewReplanner(ctx, &replanner.ReplannerConfig{
        Model:       chatModel,
        Temperature: ptrFloat32(1.0),
        MaxTokens:   ptrInt(4096),
    })
    if err != nil {
        return nil, err
    }
    
    return replannerAgent, nil
}
```

### Integration with Plan-Execute Framework

The Replanner is typically used as part of the plan-execute-replan workflow:

```go
import (
    "github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

// Create plan-execute agent with replanner
planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{
    Planner:       plannerAgent,
    Executor:      executorAgent,
    Replanner:     replannerAgent.Agent(),
    MaxIterations: 20,
})
```

## Decision Logic

### Finish Decision

The Replanner decides to **finish** when:
- All steps have been executed successfully
- The user's original request is fully satisfied
- Generated code is syntactically correct and complete
- All required files have been created/modified
- No further actions are needed

**Tool Used**: `submit_result`

### Replan Decision

The Replanner decides to **replan** when:
- Execution revealed new requirements or constraints
- A step failed and the plan needs adjustment
- The approach needs to change based on results
- Dependencies or prerequisites were discovered
- The current plan won't achieve the goal

**Tool Used**: `create_plan` with modified steps

### Continue Decision

The Replanner decides to **continue** when:
- Current step succeeded
- Remaining steps are still valid and appropriate
- Next steps logically follow from current progress
- No changes needed to the existing plan

**Tool Used**: `create_plan` with remaining steps unchanged

## Step Removal

When replanning, the Replanner automatically:
1. Removes all executed steps from the plan
2. Keeps only unexecuted steps
3. Renumbers remaining steps starting from 1
4. Preserves the overall goal

This ensures the plan stays clean and focused on remaining work.

## Termination Conditions

The replanning loop terminates when:

1. **Task Complete**: Replanner calls `submit_result`
2. **Max Iterations**: Configured iteration limit reached
3. **Execution Exception**: Unrecoverable error occurs
4. **User Termination**: User explicitly cancels (context cancellation)

## Prompt Template

The Replanner uses a structured prompt that includes:

- **Original User Request**: The user's initial query
- **Overall Goal**: The plan's objective
- **Executed Steps & Results**: What has been done and outcomes
- **Remaining Steps**: What's left to do
- **Project Context**: Relevant project information

This comprehensive context enables informed decision-making.

## Configuration

### Temperature

- **Default**: 1.0
- **Range**: 0.0 to 1.0
- **Purpose**: Controls creativity in replanning decisions
- **Recommendation**: Use 1.0 for balanced decision-making

### Max Tokens

- **Default**: 4096
- **Purpose**: Limits response size
- **Recommendation**: 4096 is sufficient for most replanning decisions

## Error Handling

The Replanner handles errors gracefully:

- **Invalid Plan Type**: Returns error if plan is not `*planner.Plan`
- **JSON Marshaling Errors**: Returns descriptive error messages
- **Prompt Formatting Errors**: Returns error with context
- **Model Errors**: Propagates model errors with additional context

## Integration Points

### With Planner

- Receives the original plan structure
- Can request plan modifications
- Maintains the same goal unless refinement needed

### With Executor

- Receives execution results
- Analyzes success/failure of steps
- Considers execution context in decisions

### With Context Manager

- Accesses project context from session
- Retrieves project ID and metadata
- Uses context for informed decisions

## Best Practices

1. **Clear Reasoning**: Always provide clear reasoning for decisions
2. **Remove Completed Steps**: Don't include executed steps in updated plans
3. **Renumber Steps**: Always renumber remaining steps from 1
4. **Preserve Goal**: Keep the same goal unless it needs refinement
5. **Be Decisive**: Don't continue if replanning is clearly needed
6. **Consider Errors**: Take execution errors seriously
7. **Validate Code**: Ensure generated code meets requirements before finishing

## Example Scenarios

### Scenario 1: Successful Completion

```
Executed Steps:
- Step 1: Create main.go file ✓
- Step 2: Implement HTTP server ✓
- Step 3: Add error handling ✓

Remaining Steps: None

Decision: FINISH
Reasoning: All steps completed successfully, HTTP server is fully implemented
```

### Scenario 2: Need to Replan

```
Executed Steps:
- Step 1: Create database schema ✓
- Step 2: Implement user model ✗ (Error: missing foreign key)

Remaining Steps:
- Step 3: Create API endpoints

Decision: REPLAN
Reasoning: Step 2 failed due to missing foreign key, need to add migration step
Updated Plan:
- Step 1: Create migration for foreign key
- Step 2: Re-implement user model with foreign key
- Step 3: Create API endpoints
```

### Scenario 3: Continue as Planned

```
Executed Steps:
- Step 1: Create project structure ✓
- Step 2: Add dependencies ✓

Remaining Steps:
- Step 3: Implement business logic
- Step 4: Add tests

Decision: CONTINUE
Reasoning: Steps 1-2 completed successfully, remaining steps are still appropriate
```

## Testing

See `replanner_test.go` for comprehensive unit tests covering:
- Replanner creation and configuration
- Decision logic for finish/replan/continue
- Step removal and renumbering
- Error handling
- Integration with plan-execute framework

## References

- [Eino ADK Documentation](https://github.com/cloudwego/eino)
- [Plan-Execute Pattern](https://github.com/cloudwego/eino/tree/main/adk/prebuilt/planexecute)
- [Integration Excel Agent Reference](https://github.com/cloudwego/eino-examples/tree/main/adk/multiagent/integration-excel-agent)
