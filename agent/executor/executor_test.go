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
	"testing"

	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// TestFormatUserInput tests the formatUserInput function
func TestFormatUserInput(t *testing.T) {
	tests := []struct {
		name     string
		messages []*schema.Message
		want     string
	}{
		{
			name:     "empty messages",
			messages: []*schema.Message{},
			want:     "No user input provided",
		},
		{
			name: "single message",
			messages: []*schema.Message{
				schema.UserMessage("Hello, world!"),
			},
			want: "[user]: Hello, world!",
		},
		{
			name: "multiple messages",
			messages: []*schema.Message{
				schema.UserMessage("Create a file"),
				schema.AssistantMessage("Sure, I'll create it", nil),
			},
			want: "[user]: Create a file\n[assistant]: Sure, I'll create it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUserInput(tt.messages)
			if got != tt.want {
				t.Errorf("formatUserInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFormatExecutedSteps tests the formatExecutedSteps function
func TestFormatExecutedSteps(t *testing.T) {
	tests := []struct {
		name  string
		steps []planexecute.ExecutedStep
		want  string
	}{
		{
			name:  "empty steps",
			steps: []planexecute.ExecutedStep{},
			want:  "No steps executed yet",
		},
		{
			name: "single step",
			steps: []planexecute.ExecutedStep{
				{
					Step:   "Create file main.go",
					Result: "File created successfully",
				},
			},
			want: "Step 1:\n  Instruction: Create file main.go\n  Result: File created successfully",
		},
		{
			name: "multiple steps",
			steps: []planexecute.ExecutedStep{
				{
					Step:   "Create file main.go",
					Result: "File created successfully",
				},
				{
					Step:   "Write code to main.go",
					Result: "Code written successfully",
				},
			},
			want: "Step 1:\n  Instruction: Create file main.go\n  Result: File created successfully\n\nStep 2:\n  Instruction: Write code to main.go\n  Result: Code written successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExecutedSteps(tt.steps)
			if got != tt.want {
				t.Errorf("formatExecutedSteps() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewExecutor tests the NewExecutor function
func TestNewExecutor(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		config  *ExecutorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "executor config cannot be nil",
		},
		{
			name: "nil model",
			config: &ExecutorConfig{
				Model: nil,
				Tools: []tool.BaseTool{},
			},
			wantErr: true,
			errMsg:  "chat model cannot be nil",
		},
		{
			name: "empty tools",
			config: &ExecutorConfig{
				Model: &mockChatModel{},
				Tools: []tool.BaseTool{},
			},
			wantErr: true,
			errMsg:  "at least one tool must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExecutor(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExecutor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("NewExecutor() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// mockChatModel is a mock implementation for testing
type mockChatModel struct{}

func (m *mockChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("mock response", nil), nil
}

func (m *mockChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *mockChatModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

func (m *mockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
