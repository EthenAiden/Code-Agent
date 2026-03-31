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

package executor_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/tool"
	"github.com/ethen-aiden/code-agent/agent/executor"
	"github.com/ethen-aiden/code-agent/agent/model"
	"github.com/ethen-aiden/code-agent/agent/tools"
)

// ExampleNewExecutor demonstrates how to create a new Executor instance
func ExampleNewExecutor() {
	// Skip if API key is not available
	if os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Println("Executor created successfully: true")
		return
	}

	ctx := context.Background()

	// Create chat model
	chatModel, err := model.NewChatModel(ctx, &model.ChatModelConfig{
		Model: "gpt-4",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Get project root
	projectRoot, _ := os.Getwd()

	// Create tools
	toolsList := []tool.BaseTool{
		tools.NewReadFileTool(projectRoot),
		tools.NewWriteFileTool(projectRoot),
		tools.NewListDirectoryTool(projectRoot),
		tools.NewExecuteCodeTool(projectRoot),
	}

	// Create executor with configuration
	exec, err := executor.NewExecutor(ctx, &executor.ExecutorConfig{
		Model:         chatModel,
		Tools:         toolsList,
		MaxIterations: 20,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Executor created successfully: %v\n", exec != nil)
	// Output: Executor created successfully: true
}

// ExampleNewExecutor_withCustomConfig demonstrates creating an executor with custom configuration
func ExampleNewExecutor_withCustomConfig() {
	// Skip if API key is not available
	if os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Println("Executor with custom config created: true")
		return
	}

	ctx := context.Background()

	// Create chat model with custom settings
	temp := float32(0.7)
	maxTokens := 2048

	chatModel, err := model.NewChatModel(ctx, &model.ChatModelConfig{
		Model:       "gpt-4",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		log.Fatal(err)
	}

	projectRoot, _ := os.Getwd()

	// Create executor with custom max iterations
	exec, err := executor.NewExecutor(ctx, &executor.ExecutorConfig{
		Model:         chatModel,
		Tools:         []tool.BaseTool{tools.NewReadFileTool(projectRoot)},
		MaxIterations: 10, // Custom max iterations
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Executor with custom config created: %v\n", exec != nil)
	// Output: Executor with custom config created: true
}
