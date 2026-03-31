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

package intent_test

import (
	"context"
	"fmt"

	"github.com/ethen-aiden/code-agent/agent/intent"
)

// ExampleIntentClassifier demonstrates basic usage of the intent classifier
func ExampleIntentClassifier() {
	// Create classifier (without LLM for this example)
	classifier := intent.NewIntentClassifier(nil)
	ctx := context.Background()

	// Example 1: Code generation query
	result1, _ := classifier.Classify(ctx, "create a REST API for user management")
	fmt.Printf("Query: 'create a REST API for user management'\n")
	fmt.Printf("Intent: %s\n", result1.Intent)
	fmt.Printf("Confidence: %.2f\n\n", result1.Confidence)

	// Example 2: Code modification query
	result2, _ := classifier.Classify(ctx, "fix the bug in the authentication logic")
	fmt.Printf("Query: 'fix the bug in the authentication logic'\n")
	fmt.Printf("Intent: %s\n", result2.Intent)
	fmt.Printf("Confidence: %.2f\n\n", result2.Confidence)

	// Example 3: Normal chat query
	result3, _ := classifier.Classify(ctx, "hello, how are you?")
	fmt.Printf("Query: 'hello, how are you?'\n")
	fmt.Printf("Intent: %s\n", result3.Intent)
	fmt.Printf("Confidence: %.2f\n", result3.Confidence)

	// Output:
	// Query: 'create a REST API for user management'
	// Intent: generate_code
	// Confidence: 0.85
	//
	// Query: 'fix the bug in the authentication logic'
	// Intent: modify_code
	// Confidence: 0.85
	//
	// Query: 'hello, how are you?'
	// Intent: chat
	// Confidence: 0.80
}

// ExampleIntentClassifier_withRouting demonstrates how to use classification for routing
func ExampleIntentClassifier_withRouting() {
	classifier := intent.NewIntentClassifier(nil)
	ctx := context.Background()

	query := "generate a login component in React"
	classification, _ := classifier.Classify(ctx, query)

	// Route based on intent
	switch classification.Intent {
	case intent.IntentChat:
		fmt.Println("Routing to: Direct Chat Response")
	case intent.IntentGenerateCode:
		fmt.Println("Routing to: Code Generation Workflow (Planner → Executor → Replanner)")
	case intent.IntentModifyCode:
		fmt.Println("Routing to: Code Modification Workflow (Planner → Executor → Replanner)")
	}

	// Output:
	// Routing to: Code Generation Workflow (Planner → Executor → Replanner)
}
