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
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	agenttools "github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/prompts"
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
	schema.SystemMessage(prompts.Load("system_executor.txt")),
	schema.UserMessage(`## OBJECTIVE
{input}

## EXECUTION PLAN
{plan}

## COMPLETED STEPS & RESULTS
{executed_steps}

## YOUR CURRENT TASK
Execute the following step:
{step}

CRITICAL: You MUST call at least ONE tool. Do NOT just describe what to do.

If step mentions "create/write" -> call write_file
If step mentions "initialize/scaffold" -> call scaffold_project
If step mentions "modify/update" -> call read_file then write_file

EXECUTE NOW USING TOOLS:`))

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

	// Build the prompt parameters
	promptParams := map[string]any{
		"input":          userInputStr,
		"plan":           string(planContent),
		"executed_steps": executedStepsStr,
		"step":           currentStep,
	}

	// Inject framework-specific constraints if known — appended to system prompt
	if fw, ok := agentcontext.GetTypedContextParams[string](ctx, "framework"); ok && fw != "" {
		if constraint := agenttools.GetFrameworkPromptConstraints(fw); constraint != "" {
			// Append framework constraint to system message by prepending a note in the user message
			promptParams["step"] = currentStep + "\n\n## Framework Constraints\n" + constraint
		}
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
