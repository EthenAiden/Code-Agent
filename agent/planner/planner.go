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
	"github.com/ethen-aiden/code-agent/prompts"
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

	prompt := prompts.Load("planner_build_planning.txt")

	if contextStr != "" {
		prompt += fmt.Sprintf("\n```json\n%s\n```\n", contextStr)
	} else {
		prompt += "\nNo additional context provided.\n"
	}

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

	// Initialize step status if not set
	for _, step := range plan.Steps {
		if step.Status == "" {
			step.Status = "pending"
		}
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

	prompt := prompts.Load("planner_build_replanning.txt")

	if contextStr != "" {
		prompt += fmt.Sprintf("\n```json\n%s\n```\n", contextStr)
	} else {
		prompt += "\nNo additional context provided.\n"
	}

	return prompt
}
