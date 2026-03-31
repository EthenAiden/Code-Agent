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

package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// IntentType represents the type of user intent
type IntentType string

const (
	// IntentChat represents normal conversation intent
	IntentChat IntentType = "chat"
	// IntentGenerateCode represents code generation intent
	IntentGenerateCode IntentType = "generate_code"
	// IntentModifyCode represents code modification intent
	IntentModifyCode IntentType = "modify_code"
)

// IntentClassification represents the result of intent classification
type IntentClassification struct {
	Intent     IntentType `json:"intent"`
	Confidence float64    `json:"confidence"`
	Reasoning  string     `json:"reasoning"`
}

// IntentClassifier classifies user queries into intent types
type IntentClassifier struct {
	chatModel model.ToolCallingChatModel
}

// NewIntentClassifier creates a new intent classifier
func NewIntentClassifier(chatModel model.ToolCallingChatModel) *IntentClassifier {
	return &IntentClassifier{
		chatModel: chatModel,
	}
}

// Classify classifies the user query into one of three intent types
func (ic *IntentClassifier) Classify(ctx context.Context, query string) (*IntentClassification, error) {
	// First, try keyword-based classification for speed
	if classification := ic.classifyByKeywords(query); classification != nil {
		return classification, nil
	}

	// Fall back to LLM-based classification for ambiguous cases
	return ic.classifyByLLM(ctx, query)
}

// classifyByKeywords performs fast keyword-based classification
func (ic *IntentClassifier) classifyByKeywords(query string) *IntentClassification {
	queryLower := strings.ToLower(query)

	// Keywords for code generation
	generateKeywords := []string{
		"create", "generate", "build", "implement", "write", "develop",
		"make", "construct", "scaffold", "initialize", "setup", "add new",
	}

	// Keywords for code modification
	modifyKeywords := []string{
		"modify", "change", "update", "fix", "refactor", "edit",
		"alter", "adjust", "improve", "optimize", "rewrite", "revise",
	}

	// Check for generate code keywords
	generateCount := 0
	for _, keyword := range generateKeywords {
		if strings.Contains(queryLower, keyword) {
			generateCount++
		}
	}

	// Check for modify code keywords
	modifyCount := 0
	for _, keyword := range modifyKeywords {
		if strings.Contains(queryLower, keyword) {
			modifyCount++
		}
	}

	// Determine intent based on keyword matches
	if generateCount > 0 && generateCount >= modifyCount {
		return &IntentClassification{
			Intent:     IntentGenerateCode,
			Confidence: 0.85,
			Reasoning:  fmt.Sprintf("Query contains code generation keywords: %d matches", generateCount),
		}
	}

	if modifyCount > 0 {
		return &IntentClassification{
			Intent:     IntentModifyCode,
			Confidence: 0.85,
			Reasoning:  fmt.Sprintf("Query contains code modification keywords: %d matches", modifyCount),
		}
	}

	// If no code-related keywords found, classify as chat
	if !containsCodeIndicators(queryLower) {
		return &IntentClassification{
			Intent:     IntentChat,
			Confidence: 0.80,
			Reasoning:  "No code-related keywords detected",
		}
	}

	// Return nil for ambiguous cases to trigger LLM classification
	return nil
}

// containsCodeIndicators checks if the query contains code-related indicators
func containsCodeIndicators(query string) bool {
	codeIndicators := []string{
		"code", "function", "class", "method", "variable", "file",
		"api", "endpoint", "component", "module", "package", "library",
		"script", "program", "application", "service", "interface",
	}

	for _, indicator := range codeIndicators {
		if strings.Contains(query, indicator) {
			return true
		}
	}

	return false
}

// classifyByLLM uses the chat model to classify ambiguous queries
func (ic *IntentClassifier) classifyByLLM(ctx context.Context, query string) (*IntentClassification, error) {
	prompt := buildClassificationPrompt(query)

	messages := []*schema.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage(query),
	}

	response, err := ic.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to classify intent using LLM: %w", err)
	}

	// Parse the LLM response
	content := response.Content
	classification, err := parseClassificationResponse(content)
	if err != nil {
		// If parsing fails, default to chat intent
		return &IntentClassification{
			Intent:     IntentChat,
			Confidence: 0.50,
			Reasoning:  "Failed to parse LLM response, defaulting to chat",
		}, nil
	}

	return classification, nil
}

// buildClassificationPrompt creates the prompt for intent classification
func buildClassificationPrompt(query string) string {
	return `You are an intent classifier for a code generation assistant. Your task is to classify user queries into one of three categories:

1. "chat" - Normal conversation, questions, or requests that don't involve code generation or modification
2. "generate_code" - Requests to create, generate, build, or implement new code
3. "modify_code" - Requests to modify, change, update, fix, or refactor existing code

Analyze the user query and respond with a JSON object in the following format:
{
  "intent": "chat" | "generate_code" | "modify_code",
  "confidence": 0.0-1.0,
  "reasoning": "Brief explanation of why this intent was chosen"
}

Guidelines:
- If the query contains keywords like "create", "generate", "build", "implement", classify as "generate_code"
- If the query contains keywords like "modify", "change", "update", "fix", "refactor", classify as "modify_code"
- If the query is a general question, conversation, or doesn't involve code work, classify as "chat"
- Provide a confidence score between 0.0 and 1.0
- Keep the reasoning brief and clear

Respond ONLY with the JSON object, no additional text.`
}

// parseClassificationResponse parses the LLM response into IntentClassification
func parseClassificationResponse(content string) (*IntentClassification, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Find JSON object boundaries
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")

	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return nil, fmt.Errorf("no valid JSON object found in response")
	}

	jsonStr := content[startIdx : endIdx+1]

	var classification IntentClassification
	if err := json.Unmarshal([]byte(jsonStr), &classification); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate intent type
	if classification.Intent != IntentChat &&
		classification.Intent != IntentGenerateCode &&
		classification.Intent != IntentModifyCode {
		return nil, fmt.Errorf("invalid intent type: %s", classification.Intent)
	}

	// Ensure confidence is in valid range
	if classification.Confidence < 0.0 {
		classification.Confidence = 0.0
	} else if classification.Confidence > 1.0 {
		classification.Confidence = 1.0
	}

	return &classification, nil
}
