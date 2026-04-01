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

package planexecute

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/agent/executor"
	"github.com/ethen-aiden/code-agent/agent/planner"
	"github.com/ethen-aiden/code-agent/agent/replanner"
	"github.com/ethen-aiden/code-agent/agent/tools"
)

// PlanExecuteConfig holds configuration for the Plan-Execute agent
type PlanExecuteConfig struct {
	// PlannerModel is used by the Planner and Replanner (GPT for reasoning).
	// Falls back to ChatModel if not set.
	PlannerModel model.ToolCallingChatModel

	// ExecutorModel is used by the Executor for code generation (Claude).
	// Falls back to ChatModel if not set.
	ExecutorModel model.ToolCallingChatModel

	// ChatModel is a legacy single-model fallback when role-specific models are not provided.
	ChatModel model.ToolCallingChatModel

	// Tools are the available tools for the executor
	Tools []tool.BaseTool

	// MaxIterations limits the maximum number of plan-execute-replan iterations
	MaxIterations int

	// PlannerTemperature controls randomness in plan generation (0.0 to 1.0)
	PlannerTemperature *float32

	// ExecutorTemperature controls randomness in execution (0.0 to 1.0)
	ExecutorTemperature *float32

	// ReplannerTemperature controls randomness in replanning (0.0 to 1.0)
	ReplannerTemperature *float32

	// MaxTokens limits the maximum tokens in responses
	MaxTokens *int
}

// plannerModel returns the model to use for the Planner/Replanner.
func (c *PlanExecuteConfig) plannerModel() model.ToolCallingChatModel {
	if c.PlannerModel != nil {
		return c.PlannerModel
	}
	return c.ChatModel
}

// executorModel returns the model to use for the Executor.
func (c *PlanExecuteConfig) executorModel() model.ToolCallingChatModel {
	if c.ExecutorModel != nil {
		return c.ExecutorModel
	}
	return c.ChatModel
}

// NewPlanExecuteAgent creates a new Plan-Execute-Replan agent
// This agent orchestrates the Planner, Executor, and Replanner in a loop
func NewPlanExecuteAgent(ctx context.Context, config *PlanExecuteConfig) (adk.Agent, error) {
	if config == nil {
		return nil, fmt.Errorf("plan-execute config cannot be nil")
	}

	if config.plannerModel() == nil {
		return nil, fmt.Errorf("planner model cannot be nil (set PlannerModel or ChatModel)")
	}
	if config.executorModel() == nil {
		return nil, fmt.Errorf("executor model cannot be nil (set ExecutorModel or ChatModel)")
	}

	if len(config.Tools) == 0 {
		return nil, fmt.Errorf("at least one tool must be provided")
	}

	// Set default max iterations if not specified
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 20
	}

	// Create Planner agent (uses GPT / PlannerModel)
	plannerAgent, err := createPlannerAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create planner agent: %w", err)
	}

	// Create Executor agent (uses Claude / ExecutorModel)
	executorAgent, err := createExecutorAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor agent: %w", err)
	}

	// Create Replanner agent (uses GPT / PlannerModel)
	replannerAgent, err := createReplannerAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create replanner agent: %w", err)
	}

	// Create the Plan-Execute agent using Eino ADK prebuilt
	planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       plannerAgent,
		Executor:      executorAgent,
		Replanner:     replannerAgent,
		MaxIterations: maxIterations,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create plan-execute agent: %w", err)
	}

	return planExecuteAgent, nil
}

// createPlannerAgent creates the Planner agent (uses PlannerModel / GPT)
func createPlannerAgent(ctx context.Context, config *PlanExecuteConfig) (adk.Agent, error) {
	// Create planner using planexecute.NewPlanner
	plannerAgent, err := planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{
		ChatModelWithFormattedOutput: config.plannerModel(),
		GenInputFn:                   generatePlannerInput,
		NewPlan: func(ctx context.Context) planexecute.Plan {
			return &planner.Plan{}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}

	return plannerAgent, nil
}

// createExecutorAgent creates the Executor agent (uses ExecutorModel / Claude)
func createExecutorAgent(ctx context.Context, config *PlanExecuteConfig) (adk.Agent, error) {
	executorConfig := &executor.ExecutorConfig{
		Model:         config.executorModel(),
		Tools:         config.Tools,
		MaxIterations: config.MaxIterations,
		Temperature:   config.ExecutorTemperature,
		MaxTokens:     config.MaxTokens,
	}

	exec, err := executor.NewExecutor(ctx, executorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return exec.Agent(), nil
}

// createReplannerAgent creates the Replanner agent (uses PlannerModel / GPT)
func createReplannerAgent(ctx context.Context, config *PlanExecuteConfig) (adk.Agent, error) {
	// Get plan tool info
	planToolInfo, err := getPlanToolInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get plan tool info: %w", err)
	}

	// Get respond tool info
	respondToolInfo := getRespondToolInfo()

	replannerConfig := &replanner.ReplannerConfig{
		Model:       config.plannerModel(),
		Temperature: config.ReplannerTemperature,
		MaxTokens:   config.MaxTokens,
		PlanTool:    planToolInfo,
		RespondTool: respondToolInfo,
	}

	replannerAgent, err := replanner.NewReplanner(ctx, replannerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create replanner: %w", err)
	}

	return replannerAgent, nil
}

// generatePlannerInput generates the input messages for the planner agent
func generatePlannerInput(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
	// Format user input
	var userInputStr string
	for i, msg := range userInput {
		if i > 0 {
			userInputStr += "\n"
		}
		userInputStr += fmt.Sprintf("[%s]: %s", msg.Role, msg.Content)
	}

	// Retrieve project context from ExecutionContext if available
	projectContextStr := ""
	if projectContext, ok := agentcontext.GetContextParams(ctx, "project_context"); ok {
		if pc, ok := projectContext.(map[string]interface{}); ok {
			projectContextBytes, err := json.MarshalIndent(pc, "", "  ")
			if err == nil {
				projectContextStr = string(projectContextBytes)
			}
		}
	}

	// Retrieve framework constraint from context (set by scaffold_tool or session metadata)
	frameworkConstraint := ""
	if fw, ok := agentcontext.GetTypedContextParams[string](ctx, "framework"); ok && fw != "" {
		frameworkConstraint = tools.GetFrameworkPromptConstraints(fw)
	}

	// Create planning prompt with project context
	systemPrompt := `You are an expert planner for a frontend code generation assistant. Your goal is to understand user requirements and break them down into a clear, step-by-step plan.

## Your Responsibilities

1. Analyze the user's request to determine the ultimate objective
2. Break down the request into granular, sequential, and executable steps
3. Ensure each step is actionable and can be executed independently
4. Order steps logically so each step builds on previous ones
5. Keep steps focused and specific

## Guidelines for Creating Steps

- Each step should be a single, clear action
- Steps should be sequential and ordered logically
- Use specific, actionable language (e.g., "Create file X with content Y")
- Avoid vague instructions (e.g., "Do something with the code")
- Consider dependencies between steps
- Include necessary context in each step description
- For a NEW project (no existing files), the FIRST step must always be: call scaffold_project tool to initialize the framework boilerplate

## Available Tools

The executor has access to the following tools:
- scaffold_project: Initialize a new project with framework boilerplate (MUST be first step for new projects)
- read_file: Read content from files in the project
- write_file: Write content to files in the project
- list_directory: List contents of directories
- run_type_check: Run TypeScript type checking (tsc --noEmit or vue-tsc --noEmit)
- run_build: Run Vite build to validate the project compiles correctly
- execute_code: Execute code in Python, JavaScript, or Go
- get_project_context: Retrieve project metadata and structure

## Output Format

You must respond by calling the create_plan tool with a JSON object containing:
{
  "goal": "Brief description of the overall objective",
  "steps": [
    {
      "id": 1,
      "description": "Clear, actionable instruction for this step",
      "executed": false
    },
    {
      "id": 2,
      "description": "Next step instruction",
      "executed": false
    }
  ]
}

## Important Notes

- Call the create_plan tool with the plan JSON
- Ensure all steps are granular and unambiguous
- Number steps sequentially starting from 1
- Set "executed" to false for all steps
- Keep the goal concise but descriptive`

	// Inject framework constraints if known
	if frameworkConstraint != "" {
		systemPrompt += "\n\n" + frameworkConstraint
	}

	// Add project context if available
	if projectContextStr != "" {
		systemPrompt += "\n\n## Project Context\n\n"
		systemPrompt += "The following project context is available for planning:\n\n"
		systemPrompt += "```json\n" + projectContextStr + "\n```\n\n"
		systemPrompt += "Use this context to create plans that integrate with the existing project structure."
	}

	systemPrompt += "\n\nNow, analyze the user's request and create an execution plan."

	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userInputStr),
	}

	return messages, nil
}

// getPlanToolInfo returns the tool info for creating plans
func getPlanToolInfo() (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_plan",
		Desc: "Create or update an execution plan with a list of steps",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"goal": {
				Type:     schema.String,
				Desc:     "Brief description of the overall objective",
				Required: true,
			},
			"steps": {
				Type:     schema.Array,
				Desc:     "List of steps to execute",
				Required: true,
			},
		}),
	}, nil
}

// getRespondToolInfo returns the tool info for submitting results
func getRespondToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "submit_result",
		Desc: "Submit the final result when the task is complete",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"result": {
				Type:     schema.String,
				Desc:     "Summary of what was accomplished",
				Required: true,
			},
		}),
	}
}
