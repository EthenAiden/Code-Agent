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
	"encoding/json"
	"fmt"
)

// Step represents a single step in an execution plan
type Step struct {
	// ID is the unique identifier for this step
	ID int `json:"id"`

	// Description is a clear, unambiguous instruction for the executor
	Description string `json:"description"`

	// Executed indicates whether this step has been completed
	Executed bool `json:"executed"`

	// Result stores the execution result for this step
	Result string `json:"result,omitempty"`

	// Status indicates the current status of the step (pending, running, done, error)
	Status string `json:"status,omitempty"`
}

// Plan represents a structured execution plan with sequential steps
type Plan struct {
	// Goal is the overall objective of the plan
	Goal string `json:"goal"`

	// Thinking contains the agent's reasoning process for creating this plan
	Thinking string `json:"thinking,omitempty"`

	// Steps is the list of steps to execute in order
	Steps []*Step `json:"steps"`
}

// NewPlan creates a new Plan with the given goal and steps
func NewPlan(goal string, steps []*Step) *Plan {
	return &Plan{
		Goal:  goal,
		Steps: steps,
	}
}

// NewPlanWithThinking creates a new Plan with goal, thinking, and steps
func NewPlanWithThinking(goal string, thinking string, steps []*Step) *Plan {
	return &Plan{
		Goal:     goal,
		Thinking: thinking,
		Steps:    steps,
	}
}

// FirstStep retrieves the first unexecuted step from the plan as a JSON string
// Returns empty string if all steps are executed or the plan is empty
// This method is required by the planexecute.Plan interface
func (p *Plan) FirstStep() string {
	if p == nil || len(p.Steps) == 0 {
		return ""
	}

	for _, step := range p.Steps {
		if !step.Executed {
			// Marshal the step to JSON string
			stepJSON, err := json.Marshal(step)
			if err != nil {
				return ""
			}
			return string(stepJSON)
		}
	}

	return ""
}

// GetFirstStep retrieves the first unexecuted step from the plan as a Step object
// Returns nil if all steps are executed or the plan is empty
func (p *Plan) GetFirstStep() *Step {
	if p == nil || len(p.Steps) == 0 {
		return nil
	}

	for _, step := range p.Steps {
		if !step.Executed {
			return step
		}
	}

	return nil
}

// RemainingSteps returns the number of unexecuted steps
func (p *Plan) RemainingSteps() int {
	if p == nil {
		return 0
	}

	count := 0
	for _, step := range p.Steps {
		if !step.Executed {
			count++
		}
	}

	return count
}

// IsComplete returns true if all steps have been executed
func (p *Plan) IsComplete() bool {
	return p.RemainingSteps() == 0
}

// MarkStepExecuted marks a step as executed with the given result
func (p *Plan) MarkStepExecuted(stepID int, result string) error {
	if p == nil {
		return fmt.Errorf("plan is nil")
	}

	for _, step := range p.Steps {
		if step.ID == stepID {
			step.Executed = true
			step.Result = result
			return nil
		}
	}

	return fmt.Errorf("step with ID %d not found", stepID)
}

// MarshalJSON implements custom JSON marshaling for Plan
func (p *Plan) MarshalJSON() ([]byte, error) {
	if p == nil {
		return json.Marshal(nil)
	}

	type Alias Plan
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for Plan.
// It handles two formats:
//  1. Eino ToolCallingChatModel path: {"steps": ["step1", "step2", ...]}  ([]string)
//  2. Custom tool path:               {"goal": "...", "steps": [{"id":1,"description":"..."},...]}
func (p *Plan) UnmarshalJSON(data []byte) error {
	// First try the full struct form ({"goal":..., "steps":[{"id":...}]}).
	type fullPlan struct {
		Goal     string  `json:"goal"`
		Thinking string  `json:"thinking,omitempty"`
		Steps    []*Step `json:"steps"`
	}
	var fp fullPlan
	if err := json.Unmarshal(data, &fp); err == nil && len(fp.Steps) > 0 {
		p.Goal = fp.Goal
		p.Thinking = fp.Thinking
		p.Steps = fp.Steps
		return nil
	}

	// Fallback: Eino's defaultPlan format — {"steps": ["string1", "string2"]}
	// Also handle when there's a "goal" key alongside string steps.
	var raw struct {
		Goal  string            `json:"goal"`
		Steps []json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	p.Goal = raw.Goal
	p.Steps = make([]*Step, 0, len(raw.Steps))
	for i, s := range raw.Steps {
		// Try object form first
		var step Step
		if err := json.Unmarshal(s, &step); err == nil && step.Description != "" {
			if step.ID == 0 {
				step.ID = i + 1
			}
			p.Steps = append(p.Steps, &step)
			continue
		}
		// Fall back to plain string
		var desc string
		if err := json.Unmarshal(s, &desc); err != nil {
			return fmt.Errorf("failed to unmarshal plan step %d: %w", i, err)
		}
		p.Steps = append(p.Steps, &Step{
			ID:          i + 1,
			Description: desc,
			Executed:    false,
		})
	}
	return nil
}

// String returns a human-readable representation of the plan
func (p *Plan) String() string {
	if p == nil {
		return "Plan: <nil>"
	}

	result := fmt.Sprintf("Plan: %s\n", p.Goal)
	result += fmt.Sprintf("Steps: %d total, %d remaining\n", len(p.Steps), p.RemainingSteps())

	for _, step := range p.Steps {
		status := "[ ]"
		if step.Executed {
			status = "[✓]"
		}
		result += fmt.Sprintf("  %s Step %d: %s\n", status, step.ID, step.Description)
	}

	return result
}
