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

// NewChatModel creates a new chat model instance supporting OpenAI-compatible APIs
// Configuration is loaded from environment variables with fallback to provided config
func NewChatModel(ctx context.Context, config *ChatModelConfig) (model.ToolCallingChatModel, error) {
	if config == nil {
		config = &ChatModelConfig{}
	}

	// Load configuration from environment variables (primary source)
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = config.BaseURL
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = config.APIKey
	}

	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = config.Model
	}

	// Validate required configuration
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if modelName == "" {
		return nil, fmt.Errorf("OPENAI_MODEL is required")
	}

	// Build OpenAI configuration
	openaiConfig := &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       modelName,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
	}

	// Create and return the chat model
	cm, err := openai.NewChatModel(ctx, openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	return cm, nil
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
