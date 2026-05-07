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
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/agent/planner"
	"github.com/ethen-aiden/code-agent/prompts"
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
	schema.SystemMessage(prompts.Load("system_replanner.txt")),
	schema.UserMessage(`## ORIGINAL USER REQUEST
{user_input}

## EXECUTION GOAL
{goal}

## EXECUTED STEPS & RESULTS
{executed_steps}

## REMAINING STEPS IN CURRENT PLAN
{remaining_steps}

{repair_context}

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

// maxRepairRounds is the maximum number of self-repair cycles before aborting.
const maxRepairRounds = 3

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

	// Detect validation failure in the last executed step and manage repair rounds.
	repairRound := 0
	if r, ok := agentcontext.GetTypedContextParams[int](ctx, "repair_round"); ok {
		repairRound = r
	}

	repairContext := ""
	if len(execCtx.ExecutedSteps) > 0 {
		lastStep := execCtx.ExecutedSteps[len(execCtx.ExecutedSteps)-1]
		if isValidationFailure(lastStep.Result) {
			repairRound++
			agentcontext.AppendContextParams(ctx, map[string]interface{}{
				"repair_round": repairRound,
			})
			if repairRound >= maxRepairRounds {
				repairContext = fmt.Sprintf(
					"## SELF-REPAIR STATUS\n⚠️ Repair round limit reached (%d/%d). Do NOT add more fix steps — call submit_result with an error summary instead.",
					repairRound, maxRepairRounds,
				)
			} else {
				repairContext = fmt.Sprintf(
					"## SELF-REPAIR STATUS\nThe last validation step FAILED (repair round %d/%d). "+
						"You MUST replan: add targeted fix steps for each error in the result above, "+
						"then add another run_type_check or run_build step to verify the fix.",
					repairRound, maxRepairRounds,
				)
			}
		} else if repairRound > 0 && !isValidationFailure(lastStep.Result) {
			// Validation passed after repairs — reset counter
			agentcontext.AppendContextParams(ctx, map[string]interface{}{
				"repair_round": 0,
			})
		}
	}

	// Generate the prompt
	messages, err := replannerPromptTemplate.Format(ctx, map[string]any{
		"user_input":      userInputStr,
		"goal":            plan.Goal,
		"executed_steps":  executedStepsStr,
		"remaining_steps": string(remainingStepsJSON),
		"repair_round":    repairRound,
		"repair_context":  repairContext,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to format replanner prompt: %w", err)
	}

	return messages, nil
}

// isValidationFailure returns true if the tool result JSON indicates a failed
// run_type_check or run_build invocation.
func isValidationFailure(result string) bool {
	result = strings.TrimSpace(result)
	if result == "" {
		return false
	}
	// Quick heuristic: the ValidationResult JSON always has "success":false when failed.
	var v struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal([]byte(result), &v); err != nil {
		return false
	}
	return !v.Success
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
