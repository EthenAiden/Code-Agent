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

package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// PlannerConfig holds configuration for the Planner agent
type PlannerConfig struct {
	// Temperature controls randomness in plan generation (0.0 to 1.0)
	Temperature *float32

	// MaxTokens limits the maximum tokens in the response
	MaxTokens *int

	// Model is the chat model used for plan generation
	Model model.ChatModel
}

// Planner is responsible for creating execution plans from user requests
type Planner struct {
	chatModel   model.ChatModel
	temperature *float32
	maxTokens   *int
}

// NewPlanner creates a new Planner instance
func NewPlanner(ctx context.Context, config *PlannerConfig) (*Planner, error) {
	if config == nil {
		return nil, fmt.Errorf("planner config cannot be nil")
	}

	if config.Model == nil {
		return nil, fmt.Errorf("chat model cannot be nil")
	}

	return &Planner{
		chatModel:   config.Model,
		temperature: config.Temperature,
		maxTokens:   config.MaxTokens,
	}, nil
}

// CreatePlan generates an execution plan from user input and context
func (p *Planner) CreatePlan(ctx context.Context, userRequest string, contextInfo map[string]interface{}) (*Plan, error) {
	if userRequest == "" {
		return nil, fmt.Errorf("user request cannot be empty")
	}

	// Build the planning prompt
	prompt := p.buildPlanningPrompt(userRequest, contextInfo)

	messages := []*schema.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage(userRequest),
	}

	// Generate plan using chat model
	response, err := p.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %w", err)
	}

	// Parse the response into a Plan
	plan, err := p.parsePlanResponse(response.Content, userRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan response: %w", err)
	}

	return plan, nil
}

// buildPlanningPrompt creates the prompt for plan generation
func (p *Planner) buildPlanningPrompt(userRequest string, contextInfo map[string]interface{}) string {
	var contextStr string
	if len(contextInfo) > 0 {
		contextBytes, _ := json.MarshalIndent(contextInfo, "", "  ")
		contextStr = string(contextBytes)
	}

	prompt := `You are a CODE GENERATION PLANNER. Create step-by-step plans for building actual, working code projects.

## Your Task

Break down the user's request into SPECIFIC, ACTIONABLE steps that will result in REAL CODE FILES being created.

## Critical Guidelines

1. **Be Specific About Files**: Each step should specify EXACTLY which file to create/modify
2. **Include Complete Code**: Steps should result in COMPLETE, WORKING code files
3. **Follow Project Structure**: Organize files properly (src/, components/, etc.)
4. **Start with Scaffold**: For new projects, first step should initialize the project structure
5. **Build Incrementally**: Create core files first, then features, then polish

## Step Format

Each step should be CONCRETE and ACTIONABLE:

❌ BAD: "Create the user interface"
✅ GOOD: "Create src/App.tsx with main application component including routing and layout"

❌ BAD: "Add styling"
✅ GOOD: "Create src/index.css with Tailwind directives and custom styles"

❌ BAD: "Implement the feature"
✅ GOOD: "Create src/components/UserList.tsx with data fetching, loading states, and error handling"

## Available Tools (Executor will use these)

- scaffold_project: Initialize project with framework boilerplate
- write_file: Write code to files
- read_file: Read existing files
- list_directory: List directory contents
- run_type_check: Validate TypeScript
- run_build: Validate project builds
- execute_code: Test code execution

## Example Plan for "Create a todo app"

{
  "goal": "Build a React TypeScript todo application with add, complete, and delete functionality",
  "steps": [
    {
      "id": 1,
      "description": "Initialize React TypeScript project structure using scaffold_project tool",
      "executed": false
    },
    {
      "id": 2,
      "description": "Create src/types/Todo.ts with TypeScript interfaces for Todo items",
      "executed": false
    },
    {
      "id": 3,
      "description": "Create src/components/TodoItem.tsx with individual todo item component including complete and delete buttons",
      "executed": false
    },
    {
      "id": 4,
      "description": "Create src/components/TodoList.tsx with list rendering and state management",
      "executed": false
    },
    {
      "id": 5,
      "description": "Create src/components/AddTodo.tsx with input form and add functionality",
      "executed": false
    },
    {
      "id": 6,
      "description": "Create src/App.tsx integrating all components with useState for todo management",
      "executed": false
    },
    {
      "id": 7,
      "description": "Create src/App.css with styling for the todo application",
      "executed": false
    }
  ]
}

## Response Format

{
  "goal": "Clear description of what will be built",
  "steps": [
    {
      "id": 1,
      "description": "Specific action with file path and what code to create",
      "executed": false
    }
  ]
}

## Context Information
`

	if contextStr != "" {
		prompt += fmt.Sprintf("\n```json\n%s\n```\n", contextStr)
	} else {
		prompt += "\nNo additional context provided.\n"
	}

	prompt += `
## Important Rules

- Respond ONLY with JSON, no additional text
- Each step must specify WHICH FILE to create/modify
- Steps should result in ACTUAL CODE FILES, not discussions
- Include 5-10 steps for most projects
- First step for new projects: scaffold_project
- Last steps: validation (run_type_check, run_build)
- Be specific about file paths and content

Now create a detailed, file-specific execution plan.`

	return prompt
}

// parsePlanResponse parses the LLM response into a Plan structure
func (p *Planner) parsePlanResponse(content string, userRequest string) (*Plan, error) {
	// Clean up the response
	content = strings.TrimSpace(content)

	// Find JSON object boundaries
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")

	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return nil, fmt.Errorf("no valid JSON object found in response")
	}

	jsonStr := content[startIdx : endIdx+1]

	// Parse JSON into Plan
	var plan Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan JSON: %w", err)
	}

	// Validate the plan
	if err := p.validatePlan(&plan); err != nil {
		return nil, fmt.Errorf("invalid plan: %w", err)
	}

	// Set goal if empty
	if plan.Goal == "" {
		plan.Goal = userRequest
	}

	return &plan, nil
}

// validatePlan ensures the plan is well-formed
func (p *Planner) validatePlan(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan must have at least one step")
	}

	// Validate each step
	seenIDs := make(map[int]bool)
	for i, step := range plan.Steps {
		if step == nil {
			return fmt.Errorf("step at index %d is nil", i)
		}

		if step.ID <= 0 {
			return fmt.Errorf("step at index %d has invalid ID: %d", i, step.ID)
		}

		if seenIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %d", step.ID)
		}
		seenIDs[step.ID] = true

		if step.Description == "" {
			return fmt.Errorf("step %d has empty description", step.ID)
		}
	}

	return nil
}

// UpdatePlan modifies an existing plan based on execution results
func (p *Planner) UpdatePlan(ctx context.Context, currentPlan *Plan, executionResults string, contextInfo map[string]interface{}) (*Plan, error) {
	if currentPlan == nil {
		return nil, fmt.Errorf("current plan cannot be nil")
	}

	// Build the replanning prompt
	prompt := p.buildReplanningPrompt(currentPlan, executionResults, contextInfo)

	messages := []*schema.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage(fmt.Sprintf("Current plan:\n%s\n\nExecution results:\n%s", currentPlan.String(), executionResults)),
	}

	// Generate updated plan using chat model
	response, err := p.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate updated plan: %w", err)
	}

	// Parse the response into an updated Plan
	updatedPlan, err := p.parsePlanResponse(response.Content, currentPlan.Goal)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated plan: %w", err)
	}

	return updatedPlan, nil
}

// buildReplanningPrompt creates the prompt for plan updates
func (p *Planner) buildReplanningPrompt(currentPlan *Plan, executionResults string, contextInfo map[string]interface{}) string {
	var contextStr string
	if len(contextInfo) > 0 {
		contextBytes, _ := json.MarshalIndent(contextInfo, "", "  ")
		contextStr = string(contextBytes)
	}

	prompt := `You are a replanning agent for a code generation assistant. Your task is to update an execution plan based on execution results.

## Your Responsibilities

1. Review the current plan and execution results
2. Identify which steps have been completed successfully
3. Determine if remaining steps need modification
4. Create an updated plan with only the remaining necessary steps
5. Adjust steps based on what was learned from execution

## Guidelines for Updating Plans

- Remove completed steps from the updated plan
- Renumber remaining steps sequentially starting from 1
- Modify step descriptions if execution revealed new information
- Add new steps if needed to complete the goal
- Keep the same goal unless it needs refinement

## Response Format

Respond with a JSON object in the following format:
{
  "goal": "The overall objective (same or refined)",
  "steps": [
    {
      "id": 1,
      "description": "Updated or new step instruction",
      "executed": false
    }
  ]
}

## Context Information
`

	if contextStr != "" {
		prompt += fmt.Sprintf("\n```json\n%s\n```\n", contextStr)
	} else {
		prompt += "\nNo additional context provided.\n"
	}

	prompt += `
## Important Notes

- Respond ONLY with the JSON object, no additional text
- Only include unexecuted steps in the updated plan
- Renumber steps starting from 1
- Set "executed" to false for all steps
- Consider execution results when updating steps

Now, review the current plan and execution results, then create an updated plan.`

	return prompt
}
