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

package replanner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/ethen-aiden/code-agent/agent/planner"
)

// ReplannerConfig holds configuration for the Replanner agent
type ReplannerConfig struct {
	// Model is the chat model used for replanning
	Model model.ToolCallingChatModel

	// Temperature controls randomness in replanning (0.0 to 1.0)
	Temperature *float32

	// MaxTokens limits the maximum tokens in the response
	MaxTokens *int

	// PlanTool is the tool info for creating plans
	PlanTool *schema.ToolInfo

	// RespondTool is the tool info for submitting results
	RespondTool *schema.ToolInfo
}

// replannerPromptTemplate defines the prompt template for the replanner agent
var replannerPromptTemplate = prompt.FromMessages(schema.FString,
	schema.SystemMessage(`You are a meticulous replanner agent for a code generation assistant. Your role is to evaluate execution progress and decide the next action.

## Your Responsibilities

1. Review the original user request and goal
2. Analyze executed steps and their results
3. Evaluate remaining steps in the current plan
4. Decide whether to:
   - FINISH: Task is complete, submit final result
   - REPLAN: Current plan needs modification, create updated plan
   - CONTINUE: Current plan is good, continue with remaining steps

## Decision Guidelines

### When to FINISH
- All steps have been executed successfully
- The user's goal has been achieved
- Generated code is syntactically correct and complete
- All required files have been created/modified
- No further action is needed

### When to REPLAN
- Execution revealed unexpected issues or errors
- Current plan is insufficient to achieve the goal
- Steps need to be added, removed, or modified
- Execution results suggest a different approach
- Dependencies or requirements changed

### When to CONTINUE
- Execution is progressing as expected
- Remaining steps are still appropriate
- No adjustments needed to the current plan
- Simply proceed with the next step

## Available Tools

You have access to two tools:
1. create_plan: Create a new or updated plan with steps
2. submit_result: Submit the final result when task is complete

## Important Notes

- Always consider the original user request when making decisions
- Review ALL executed steps and their results carefully
- If replanning, remove completed steps and renumber remaining steps
- If finishing, provide a clear summary of what was accomplished
- Be decisive - don't replan unnecessarily if the current plan is working
- Ensure generated code meets quality standards before finishing`),
	schema.UserMessage(`## ORIGINAL USER REQUEST
{user_input}

## EXECUTION GOAL
{goal}

## EXECUTED STEPS & RESULTS
{executed_steps}

## REMAINING STEPS IN CURRENT PLAN
{remaining_steps}

## YOUR TASK
Evaluate the execution progress and decide the next action:
- If the task is complete, use submit_result tool
- If the plan needs modification, use create_plan tool with updated steps
- If the plan is good, use create_plan tool with remaining steps unchanged

Make your decision based on the execution results and remaining work.`))

// NewReplanner creates a new Replanner agent
func NewReplanner(ctx context.Context, config *ReplannerConfig) (adk.Agent, error) {
	if config == nil {
		return nil, fmt.Errorf("replanner config cannot be nil")
	}

	if config.Model == nil {
		return nil, fmt.Errorf("chat model cannot be nil")
	}

	// Create the replanner agent using planexecute.NewReplanner
	replannerAgent, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:   config.Model,
		PlanTool:    config.PlanTool,
		RespondTool: config.RespondTool,
		GenInputFn:  generateReplannerInput,
		NewPlan: func(ctx context.Context) planexecute.Plan {
			return &planner.Plan{}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create replanner agent: %w", err)
	}

	return replannerAgent, nil
}

// generateReplannerInput generates the input messages for the replanner agent
func generateReplannerInput(ctx context.Context, execCtx *planexecute.ExecutionContext) ([]adk.Message, error) {
	// Get the plan
	plan, ok := execCtx.Plan.(*planner.Plan)
	if !ok {
		return nil, fmt.Errorf("plan is not of type *planner.Plan")
	}

	// Format user input
	userInputStr := formatUserInput(execCtx.UserInput)

	// Format executed steps
	executedStepsStr := formatExecutedSteps(execCtx.ExecutedSteps)

	// Format remaining steps (remove the first step which was just executed)
	remainingPlan := &planner.Plan{
		Goal:  plan.Goal,
		Steps: plan.Steps,
	}
	if len(remainingPlan.Steps) > 0 {
		remainingPlan.Steps = remainingPlan.Steps[1:]
	}
	remainingStepsJSON, err := json.MarshalIndent(remainingPlan, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal remaining steps: %w", err)
	}

	// Generate the prompt
	messages, err := replannerPromptTemplate.Format(ctx, map[string]any{
		"user_input":      userInputStr,
		"goal":            plan.Goal,
		"executed_steps":  executedStepsStr,
		"remaining_steps": string(remainingStepsJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to format replanner prompt: %w", err)
	}

	return messages, nil
}

// formatUserInput formats the user input messages into a string
func formatUserInput(messages []*schema.Message) string {
	if len(messages) == 0 {
		return "No user input provided"
	}

	var result string
	for i, msg := range messages {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("[%s]: %s", msg.Role, msg.Content)
	}

	return result
}

// formatExecutedSteps formats the executed steps into a readable string
func formatExecutedSteps(steps []planexecute.ExecutedStep) string {
	if len(steps) == 0 {
		return "No steps executed yet"
	}

	var result string
	for i, step := range steps {
		if i > 0 {
			result += "\n\n"
		}
		result += fmt.Sprintf("Step %d:\n", i+1)
		result += fmt.Sprintf("  Instruction: %s\n", step.Step)
		result += fmt.Sprintf("  Result: %s", step.Result)
	}

	return result
}
