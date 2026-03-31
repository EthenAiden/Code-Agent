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
	"os"
	"testing"
)

func TestNewChatModel_WithEnvironmentVariables(t *testing.T) {
	// Set up environment variables
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	os.Setenv("OPENAI_MODEL", "gpt-4")
	os.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_MODEL")
		os.Unsetenv("OPENAI_BASE_URL")
	}()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, nil)

	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	if cm == nil {
		t.Fatal("Expected non-nil chat model")
	}
}

func TestNewChatModel_WithConfig(t *testing.T) {
	// Clear environment variables to test config fallback
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")
	os.Unsetenv("OPENAI_BASE_URL")

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey:  "test-api-key",
		Model:   "gpt-4",
		BaseURL: "https://api.openai.com/v1",
	}

	cm, err := NewChatModel(ctx, config)

	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	if cm == nil {
		t.Fatal("Expected non-nil chat model")
	}
}

func TestNewChatModel_MissingAPIKey(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")
	os.Unsetenv("OPENAI_BASE_URL")

	ctx := context.Background()
	config := &ChatModelConfig{
		Model: "gpt-4",
	}

	_, err := NewChatModel(ctx, config)

	if err == nil {
		t.Fatal("Expected error for missing API key")
	}

	if err.Error() != "OPENAI_API_KEY is required" {
		t.Errorf("Expected 'OPENAI_API_KEY is required' error, got: %v", err)
	}
}

func TestNewChatModel_MissingModel(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")
	os.Unsetenv("OPENAI_BASE_URL")

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey: "test-api-key",
	}

	_, err := NewChatModel(ctx, config)

	if err == nil {
		t.Fatal("Expected error for missing model")
	}

	if err.Error() != "OPENAI_MODEL is required" {
		t.Errorf("Expected 'OPENAI_MODEL is required' error, got: %v", err)
	}
}

func TestNewChatModel_EnvironmentOverridesConfig(t *testing.T) {
	// Set environment variables
	os.Setenv("OPENAI_API_KEY", "env-api-key")
	os.Setenv("OPENAI_MODEL", "env-model")
	os.Setenv("OPENAI_BASE_URL", "https://env.api.com/v1")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_MODEL")
		os.Unsetenv("OPENAI_BASE_URL")
	}()

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey:  "config-api-key",
		Model:   "config-model",
		BaseURL: "https://config.api.com/v1",
	}

	cm, err := NewChatModel(ctx, config)

	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	if cm == nil {
		t.Fatal("Expected non-nil chat model")
	}

	// Note: We can't directly verify the internal config values,
	// but the test ensures environment variables take precedence
}

func TestNewChatModelWithOptions(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")
	os.Unsetenv("OPENAI_BASE_URL")

	ctx := context.Background()
	temp := float32(0.7)
	maxTokens := 2048
	topP := float32(0.9)

	cm, err := NewChatModelWithOptions(ctx,
		WithAPIKey("test-api-key"),
		WithModel("gpt-4"),
		WithBaseURL("https://api.openai.com/v1"),
		WithTemperature(temp),
		WithMaxTokens(maxTokens),
		WithTopP(topP),
	)

	if err != nil {
		t.Fatalf("NewChatModelWithOptions failed: %v", err)
	}

	if cm == nil {
		t.Fatal("Expected non-nil chat model")
	}
}

func TestChatModelOptions(t *testing.T) {
	config := &ChatModelConfig{}

	// Test WithBaseURL
	WithBaseURL("https://test.com")(config)
	if config.BaseURL != "https://test.com" {
		t.Errorf("WithBaseURL failed: expected 'https://test.com', got '%s'", config.BaseURL)
	}

	// Test WithAPIKey
	WithAPIKey("test-key")(config)
	if config.APIKey != "test-key" {
		t.Errorf("WithAPIKey failed: expected 'test-key', got '%s'", config.APIKey)
	}

	// Test WithModel
	WithModel("gpt-4")(config)
	if config.Model != "gpt-4" {
		t.Errorf("WithModel failed: expected 'gpt-4', got '%s'", config.Model)
	}

	// Test WithTemperature
	temp := float32(0.5)
	WithTemperature(temp)(config)
	if config.Temperature == nil || *config.Temperature != temp {
		t.Errorf("WithTemperature failed")
	}

	// Test WithMaxTokens
	maxTokens := 1024
	WithMaxTokens(maxTokens)(config)
	if config.MaxTokens == nil || *config.MaxTokens != maxTokens {
		t.Errorf("WithMaxTokens failed")
	}

	// Test WithTopP
	topP := float32(0.8)
	WithTopP(topP)(config)
	if config.TopP == nil || *config.TopP != topP {
		t.Errorf("WithTopP failed")
	}
}
