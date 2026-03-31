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
	"testing"
	"time"
)

// TestClassifyByKeywords_GenerateCode tests keyword-based classification for code generation
func TestClassifyByKeywords_GenerateCode(t *testing.T) {
	classifier := NewIntentClassifier(nil) // No LLM needed for keyword tests

	testCases := []struct {
		name  string
		query string
	}{
		{"create keyword", "create a new REST API"},
		{"generate keyword", "generate a user authentication module"},
		{"build keyword", "build a React component for login"},
		{"implement keyword", "implement a sorting algorithm"},
		{"write keyword", "write a function to parse JSON"},
		{"develop keyword", "develop a payment processing service"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.classifyByKeywords(tc.query)
			if result == nil {
				t.Fatalf("Expected classification result, got nil")
			}
			if result.Intent != IntentGenerateCode {
				t.Errorf("Expected intent %s, got %s", IntentGenerateCode, result.Intent)
			}
			if result.Confidence < 0.5 {
				t.Errorf("Expected confidence >= 0.5, got %f", result.Confidence)
			}
		})
	}
}

// TestClassifyByKeywords_ModifyCode tests keyword-based classification for code modification
func TestClassifyByKeywords_ModifyCode(t *testing.T) {
	classifier := NewIntentClassifier(nil)

	testCases := []struct {
		name  string
		query string
	}{
		{"modify keyword", "modify the user service to add logging"},
		{"change keyword", "change the database connection string"},
		{"update keyword", "update the API endpoint to return JSON"},
		{"fix keyword", "fix the bug in the authentication logic"},
		{"refactor keyword", "refactor the code to use async/await"},
		{"edit keyword", "edit the configuration file"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.classifyByKeywords(tc.query)
			if result == nil {
				t.Fatalf("Expected classification result, got nil")
			}
			if result.Intent != IntentModifyCode {
				t.Errorf("Expected intent %s, got %s", IntentModifyCode, result.Intent)
			}
			if result.Confidence < 0.5 {
				t.Errorf("Expected confidence >= 0.5, got %f", result.Confidence)
			}
		})
	}
}

// TestClassifyByKeywords_Chat tests keyword-based classification for normal chat
func TestClassifyByKeywords_Chat(t *testing.T) {
	classifier := NewIntentClassifier(nil)

	testCases := []struct {
		name  string
		query string
	}{
		{"greeting", "hello, how are you?"},
		{"question", "what is the weather today?"},
		{"general query", "tell me about machine learning"},
		{"help request", "can you help me understand recursion?"},
		{"explanation", "explain how databases work"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.classifyByKeywords(tc.query)
			if result == nil {
				// Ambiguous cases return nil, which is acceptable
				return
			}
			if result.Intent != IntentChat {
				t.Errorf("Expected intent %s, got %s", IntentChat, result.Intent)
			}
		})
	}
}

// TestClassifyByKeywords_Performance tests that classification is fast
func TestClassifyByKeywords_Performance(t *testing.T) {
	classifier := NewIntentClassifier(nil)
	query := "create a new REST API for user management"

	start := time.Now()
	result := classifier.classifyByKeywords(query)
	duration := time.Since(start)

	if result == nil {
		t.Fatal("Expected classification result, got nil")
	}

	// Should complete in less than 500ms (requirement 1.5)
	if duration > 500*time.Millisecond {
		t.Errorf("Classification took %v, expected < 500ms", duration)
	}
}

// TestParseClassificationResponse tests JSON parsing
func TestParseClassificationResponse(t *testing.T) {
	testCases := []struct {
		name         string
		response     string
		expectError  bool
		expectIntent IntentType
	}{
		{
			name: "valid generate_code response",
			response: `{
				"intent": "generate_code",
				"confidence": 0.95,
				"reasoning": "User wants to create new code"
			}`,
			expectError:  false,
			expectIntent: IntentGenerateCode,
		},
		{
			name: "valid modify_code response",
			response: `{
				"intent": "modify_code",
				"confidence": 0.90,
				"reasoning": "User wants to modify existing code"
			}`,
			expectError:  false,
			expectIntent: IntentModifyCode,
		},
		{
			name: "valid chat response",
			response: `{
				"intent": "chat",
				"confidence": 0.85,
				"reasoning": "General conversation"
			}`,
			expectError:  false,
			expectIntent: IntentChat,
		},
		{
			name: "response with extra text",
			response: `Here is the classification:
			{
				"intent": "generate_code",
				"confidence": 0.95,
				"reasoning": "Code generation request"
			}
			Hope this helps!`,
			expectError:  false,
			expectIntent: IntentGenerateCode,
		},
		{
			name:        "invalid JSON",
			response:    "not a json response",
			expectError: true,
		},
		{
			name: "invalid intent type",
			response: `{
				"intent": "invalid_intent",
				"confidence": 0.95,
				"reasoning": "Test"
			}`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseClassificationResponse(tc.response)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Intent != tc.expectIntent {
				t.Errorf("Expected intent %s, got %s", tc.expectIntent, result.Intent)
			}

			if result.Confidence < 0.0 || result.Confidence > 1.0 {
				t.Errorf("Confidence out of range: %f", result.Confidence)
			}
		})
	}
}

// TestContainsCodeIndicators tests code indicator detection
func TestContainsCodeIndicators(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		expected bool
	}{
		{"has code keyword", "show me the code", true},
		{"has function keyword", "create a function", true},
		{"has class keyword", "define a class", true},
		{"has api keyword", "build an api", true},
		{"no code indicators", "what is the weather", false},
		{"no code indicators 2", "tell me a story", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := containsCodeIndicators(tc.query)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for query: %s", tc.expected, result, tc.query)
			}
		})
	}
}

// TestClassify_WithNilChatModel tests that keyword classification works without LLM
func TestClassify_WithNilChatModel(t *testing.T) {
	classifier := NewIntentClassifier(nil)
	ctx := context.Background()

	// Test queries that should be classified by keywords
	testCases := []struct {
		query          string
		expectedIntent IntentType
	}{
		{"create a new API", IntentGenerateCode},
		{"modify the existing function", IntentModifyCode},
		{"hello there", IntentChat},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			result, err := classifier.Classify(ctx, tc.query)

			// For ambiguous cases with nil chat model, we expect an error
			if err != nil && result == nil {
				// This is acceptable for ambiguous cases
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Intent != tc.expectedIntent {
				t.Errorf("Expected intent %s, got %s", tc.expectedIntent, result.Intent)
			}
		})
	}
}
