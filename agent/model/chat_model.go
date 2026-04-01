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

package model

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// ChatModelConfig holds configuration for creating a chat model
type ChatModelConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature *float32
	MaxTokens   *int
	TopP        *float32
}

// ModelRole identifies which role a model serves in the agent pipeline.
type ModelRole string

const (
	// ModelRolePlanner is used by IntentClassifier, Planner, and Replanner.
	// Configured via PLANNER_* env vars (defaults to OPENAI_* for backward compat).
	ModelRolePlanner ModelRole = "planner"

	// ModelRoleExecutor is used by the Executor for code generation.
	// Configured via CLAUDE_* env vars (defaults to OPENAI_* for backward compat).
	ModelRoleExecutor ModelRole = "executor"
)

// NewChatModel creates a new chat model instance supporting OpenAI-compatible APIs.
// Configuration is loaded from environment variables with fallback to provided config.
func NewChatModel(ctx context.Context, config *ChatModelConfig) (model.ToolCallingChatModel, error) {
	return newChatModelForRole(ctx, config, "OPENAI")
}

// NewPlannerModel creates the model used by IntentClassifier, Planner, and Replanner.
// Reads PLANNER_* env vars, falls back to OPENAI_* for backward compatibility.
func NewPlannerModel(ctx context.Context, config *ChatModelConfig) (model.ToolCallingChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}
	// Try PLANNER_* prefix first, then fall back to OPENAI_*
	apiKey := firstNonEmpty(os.Getenv("PLANNER_API_KEY"), os.Getenv("OPENAI_API_KEY"), config.APIKey)
	baseURL := firstNonEmpty(os.Getenv("PLANNER_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), config.BaseURL)
	modelName := firstNonEmpty(os.Getenv("PLANNER_MODEL"), os.Getenv("OPENAI_MODEL"), config.Model)
	return buildChatModel(ctx, apiKey, baseURL, modelName, config)
}

// NewExecutorModel creates the model used by the Executor for code generation.
// Reads CLAUDE_* env vars, falls back to OPENAI_* for backward compatibility.
func NewExecutorModel(ctx context.Context, config *ChatModelConfig) (model.ToolCallingChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}
	// Try CLAUDE_* prefix first, then fall back to OPENAI_*
	apiKey := firstNonEmpty(os.Getenv("CLAUDE_API_KEY"), os.Getenv("OPENAI_API_KEY"), config.APIKey)
	baseURL := firstNonEmpty(os.Getenv("CLAUDE_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), config.BaseURL)
	modelName := firstNonEmpty(os.Getenv("CLAUDE_MODEL"), os.Getenv("OPENAI_MODEL"), config.Model)
	return buildChatModel(ctx, apiKey, baseURL, modelName, config)
}

// newChatModelForRole is a helper that reads the given env prefix (e.g. "OPENAI").
func newChatModelForRole(ctx context.Context, config *ChatModelConfig, prefix string) (model.ToolCallingChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}
	apiKey := firstNonEmpty(os.Getenv(prefix+"_API_KEY"), config.APIKey)
	baseURL := firstNonEmpty(os.Getenv(prefix+"_BASE_URL"), config.BaseURL)
	modelName := firstNonEmpty(os.Getenv(prefix+"_MODEL"), config.Model)
	return buildChatModel(ctx, apiKey, baseURL, modelName, config)
}

// buildChatModel constructs an openai-compatible ChatModel from resolved credentials.
func buildChatModel(ctx context.Context, apiKey, baseURL, modelName string, config *ChatModelConfig) (model.ToolCallingChatModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required (set OPENAI_API_KEY, PLANNER_API_KEY, or CLAUDE_API_KEY)")
	}
	if modelName == "" {
		return nil, fmt.Errorf("model name is required (set OPENAI_MODEL, PLANNER_MODEL, or CLAUDE_MODEL)")
	}

	openaiConfig := &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       modelName,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
	}

	cm, err := openai.NewChatModel(ctx, openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}
	return cm, nil
}

// firstNonEmpty returns the first non-empty string from the candidates.
func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if s != "" {
			return s
		}
	}
	return ""
}

// ChatModelOption is a functional option for configuring chat model
type ChatModelOption func(*ChatModelConfig)

// WithBaseURL sets the base URL for the chat model API
func WithBaseURL(baseURL string) ChatModelOption {
	return func(c *ChatModelConfig) {
		c.BaseURL = baseURL
	}
}

// WithAPIKey sets the API key for the chat model
func WithAPIKey(apiKey string) ChatModelOption {
	return func(c *ChatModelConfig) {
		c.APIKey = apiKey
	}
}

// WithModel sets the model name
func WithModel(model string) ChatModelOption {
	return func(c *ChatModelConfig) {
		c.Model = model
	}
}

// WithTemperature sets the temperature parameter
func WithTemperature(temp float32) ChatModelOption {
	return func(c *ChatModelConfig) {
		c.Temperature = &temp
	}
}

// WithMaxTokens sets the max tokens parameter
func WithMaxTokens(maxTokens int) ChatModelOption {
	return func(c *ChatModelConfig) {
		c.MaxTokens = &maxTokens
	}
}

// WithTopP sets the top_p parameter
func WithTopP(topP float32) ChatModelOption {
	return func(c *ChatModelConfig) {
		c.TopP = &topP
	}
}

// NewChatModelWithOptions creates a chat model with functional options
func NewChatModelWithOptions(ctx context.Context, opts ...ChatModelOption) (model.ToolCallingChatModel, error) {
	config := &ChatModelConfig{}
	for _, opt := range opts {
		opt(config)
	}
	return NewChatModel(ctx, config)
}
