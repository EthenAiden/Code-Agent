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
	agenttools "github.com/ethen-aiden/code-agent/agent/tools"
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
	schema.SystemMessage(`You are a CODE GENERATION EXECUTOR. Your ONLY job is to WRITE ACTUAL CODE FILES using tools.

## ⚠️ CRITICAL RULE: YOU MUST CALL TOOLS - NO EXCEPTIONS

You are FORBIDDEN from just describing code or explaining what to do. You MUST:
1. Call the appropriate tool (write_file, scaffold_project, etc.)
2. Pass the complete code as parameters
3. Wait for tool execution result
4. Report success/failure

## 🚫 FORBIDDEN BEHAVIORS

❌ "Here's the code you need: [code snippet]"
❌ "You should create a file with this content..."
❌ "The App.tsx should look like this..."
❌ Explaining code without calling write_file
❌ Showing code examples without saving them

## ✅ REQUIRED BEHAVIOR

✅ Call write_file tool with path and complete code
✅ Call scaffold_project for new projects
✅ Call read_file before modifying existing files
✅ Report tool execution results

## Available Tools

- scaffold_project: Initialize project (REQUIRED for new projects)
- write_file: Write code to file (REQUIRED for all code generation)
- read_file: Read existing files
- list_directory: List directory contents
- run_type_check: Validate TypeScript
- run_build: Validate builds
- execute_code: Test code
- get_project_context: Get project info

## Step-by-Step Execution Pattern

### For "Create src/App.tsx with React component":

Step 1: Generate the complete code in your mind
Step 2: IMMEDIATELY call write_file tool:
{
  "path": "src/App.tsx",
  "content": "import React from 'react';\n\nfunction App() {\n  return (\n    <div className=\"App\">\n      <h1>Hello World</h1>\n    </div>\n  );\n}\n\nexport default App;"
}
Step 3: Wait for tool result
Step 4: Report: "✓ Created src/App.tsx with React component"

### For "Initialize React project":

Step 1: Call scaffold_project tool:
{
  "framework": "react",
  "project_name": "my-app"
}
Step 2: Wait for tool result
Step 3: Report: "✓ Initialized React project structure"

## Real Examples

Example 1 - Creating a component:
USER STEP: "Create src/components/Button.tsx with a reusable button component"
YOUR ACTION: Call write_file with:
{
  "path": "src/components/Button.tsx",
  "content": "import React from 'react';\n\ninterface ButtonProps {\n  onClick: () => void;\n  children: React.ReactNode;\n  variant?: 'primary' | 'secondary';\n}\n\nexport function Button({ onClick, children, variant = 'primary' }: ButtonProps) {\n  return (\n    <button\n      onClick={onClick}\n      className={variant === 'primary' ? 'btn-primary' : 'btn-secondary'}\n    >\n      {children}\n    </button>\n  );\n}"
}
YOUR RESPONSE: "✓ Created src/components/Button.tsx with reusable Button component including TypeScript props and variants"

Example 2 - Modifying existing file:
USER STEP: "Add a counter state to src/App.tsx"
YOUR ACTION 1: Call read_file with path="src/App.tsx"
YOUR ACTION 2: Call write_file with modified content including useState
YOUR RESPONSE: "✓ Updated src/App.tsx to include counter state with useState hook"

## Code Quality Standards

- COMPLETE code (all imports, types, exports)
- WORKING code (no placeholders or TODOs)
- PROPER formatting (consistent indentation)
- FRAMEWORK conventions (React hooks, Vue composition API, etc.)
- TYPESCRIPT types (interfaces, type annotations)
- ERROR handling where appropriate

## Tool Call Format

When calling write_file, use this exact format:
{
  "path": "relative/path/to/file.tsx",
  "content": "complete file content here"
}

When calling scaffold_project:
{
  "framework": "react" | "vue3" | "react-native",
  "project_name": "project-name"
}

## Response Format After Tool Call

After EVERY tool call, respond with:
"✓ [Action completed] - [Brief description]"

Example:
"✓ Created src/App.tsx - Main application component with routing"
"✓ Updated src/components/Header.tsx - Added navigation menu"
"✓ Initialized React project - Created project structure with Vite"

## Remember

- You are an EXECUTOR, not an EXPLAINER
- TOOLS are your ONLY way to accomplish tasks
- EVERY code generation step REQUIRES a write_file call
- NO EXCEPTIONS to this rule
{framework_constraints}`),
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

⚠️ CRITICAL: You MUST call at least ONE tool. Do NOT just describe what to do.

If step mentions "create/write" → call write_file
If step mentions "initialize/scaffold" → call scaffold_project  
If step mentions "modify/update" → call read_file then write_file

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
		"input":                 userInputStr,
		"plan":                  string(planContent),
		"executed_steps":        executedStepsStr,
		"step":                  currentStep,
		"framework_constraints": "",
	}

	// Inject framework-specific constraints if known
	if fw, ok := agentcontext.GetTypedContextParams[string](ctx, "framework"); ok && fw != "" {
		if constraint := agenttools.GetFrameworkPromptConstraints(fw); constraint != "" {
			promptParams["framework_constraints"] = "\n\n" + constraint
		}
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
