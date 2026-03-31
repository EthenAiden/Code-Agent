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

package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
)

// ExecutorConfig holds configuration for the Executor agent
type ExecutorConfig struct {
	// Model is the chat model used for step execution
	Model model.ToolCallingChatModel

	// Tools are the available tools for the executor to use
	Tools []tool.BaseTool

	// MaxIterations limits the maximum number of iterations per step execution
	MaxIterations int

	// Temperature controls randomness in execution (0.0 to 1.0)
	Temperature *float32

	// MaxTokens limits the maximum tokens in the response
	MaxTokens *int
}

// Executor is responsible for executing individual steps from a plan
type Executor struct {
	agent adk.Agent
}

// executorPrompt defines the prompt template for the executor agent
var executorPrompt = prompt.FromMessages(schema.FString,
	schema.SystemMessage(`You are a diligent and meticulous executor agent for a code generation assistant. Your role is to execute individual steps from a plan using available tools.

## Your Responsibilities

1. Execute the given step precisely and completely
2. Use available tools to accomplish the step
3. Generate syntactically correct code when required
4. Return clear execution results
5. Handle errors gracefully with descriptive messages

## Available Tools

You have access to the following tools:
- read_file: Read content from files in the project
- write_file: Write content to files in the project
- list_directory: List contents of directories
- execute_code: Execute code in Python, JavaScript, or Go
- get_project_context: Retrieve project metadata and structure

## Guidelines for Execution

- Follow the step instruction exactly
- Use tools appropriately for each task
- When generating code:
  * Ensure syntax is correct for the target language
  * Include necessary imports and dependencies
  * Add comments for clarity
  * Follow language-specific best practices
  * Consider the project context and integrate with existing code structure
- When writing files:
  * Use appropriate file paths
  * Ensure content is properly formatted
  * Verify the operation succeeded
- When reading files:
  * Handle missing files gracefully
  * Process content appropriately
- Report execution results clearly and concisely

## Important Notes

- Execute only the current step, do not skip ahead
- Use tools to accomplish tasks, don't just describe what to do
- If a step cannot be completed, explain why clearly
- Always verify your work when possible
- Generate production-quality code, not pseudocode
- Use project context to ensure generated code integrates properly`),
	schema.UserMessage(`## OBJECTIVE
{input}

## EXECUTION PLAN
{plan}

## COMPLETED STEPS & RESULTS
{executed_steps}

{{if .project_context}}## PROJECT CONTEXT
The following project context is available:
`+"```json\n{project_context}\n```"+`

Use this context to ensure your work integrates with the existing project structure.
{{end}}

## YOUR CURRENT TASK
Execute the following step:
{step}

Use the available tools to complete this step. Generate syntactically correct code if needed.`))

// NewExecutor creates a new Executor instance
func NewExecutor(ctx context.Context, config *ExecutorConfig) (*Executor, error) {
	if config == nil {
		return nil, fmt.Errorf("executor config cannot be nil")
	}

	if config.Model == nil {
		return nil, fmt.Errorf("chat model cannot be nil")
	}

	if len(config.Tools) == 0 {
		return nil, fmt.Errorf("at least one tool must be provided")
	}

	// Set default max iterations if not specified
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 20
	}

	// Create the executor agent using planexecute.NewExecutor
	executorAgent, err := planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: config.Model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: config.Tools,
			},
		},
		MaxIterations: maxIterations,
		GenInputFn:    generateExecutorInput,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create executor agent: %w", err)
	}

	return &Executor{
		agent: executorAgent,
	}, nil
}

// generateExecutorInput generates the input messages for the executor agent
func generateExecutorInput(ctx context.Context, execCtx *planexecute.ExecutionContext) ([]adk.Message, error) {
	// Marshal the plan to JSON for display
	planContent, err := execCtx.Plan.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Format executed steps
	executedStepsStr := formatExecutedSteps(execCtx.ExecutedSteps)

	// Get the current step to execute
	currentStep := execCtx.Plan.FirstStep()
	if currentStep == "" {
		return nil, fmt.Errorf("no step to execute")
	}

	// Format the user input
	userInputStr := formatUserInput(execCtx.UserInput)

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

	// Build the prompt parameters
	promptParams := map[string]any{
		"input":          userInputStr,
		"plan":           string(planContent),
		"executed_steps": executedStepsStr,
		"step":           currentStep,
	}

	// Add project context if available
	if projectContextStr != "" {
		promptParams["project_context"] = projectContextStr
	}

	// Generate the prompt
	messages, err := executorPrompt.Format(ctx, promptParams)
	if err != nil {
		return nil, fmt.Errorf("failed to format executor prompt: %w", err)
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

// Agent returns the underlying ADK agent
func (e *Executor) Agent() adk.Agent {
	return e.agent
}

// Execute runs the executor agent with the given execution context
func (e *Executor) Execute(ctx context.Context, execCtx *planexecute.ExecutionContext) (*schema.Message, error) {
	if e.agent == nil {
		return nil, fmt.Errorf("executor agent is not initialized")
	}

	// The executor agent is invoked through the plan-execute framework
	// This method is provided for direct invocation if needed
	return nil, fmt.Errorf("direct execution not supported, use through plan-execute framework")
}
