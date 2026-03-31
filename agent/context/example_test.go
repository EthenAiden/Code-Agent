package context_test

import (
	"context"
	"fmt"

	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
)

// ExampleInitContextParams demonstrates how to initialize a context with parameter storage
func ExampleInitContextParams() {
	ctx := context.Background()
	ctx = agentcontext.InitContextParams(ctx)

	// Now the context is ready to store parameters
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": "my-project-123",
	})

	projectID, ok := agentcontext.GetTypedContextParams[string](ctx, "project_id")
	if ok {
		fmt.Println(projectID)
	}
	// Output: my-project-123
}

// ExampleAppendContextParams demonstrates how to add parameters to the context
func ExampleAppendContextParams() {
	ctx := context.Background()
	ctx = agentcontext.InitContextParams(ctx)

	// Add single parameter
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": "project-123",
	})

	// Add multiple parameters
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"user_id":  "user-456",
		"work_dir": "/path/to/project",
	})

	// Update existing parameter
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": "updated-project-789",
	})

	projectID, _ := agentcontext.GetTypedContextParams[string](ctx, "project_id")
	fmt.Println(projectID)
	// Output: updated-project-789
}

// ExampleGetTypedContextParams demonstrates type-safe parameter retrieval
func ExampleGetTypedContextParams() {
	ctx := context.Background()
	ctx = agentcontext.InitContextParams(ctx)

	// Store different types
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": "project-123",
		"count":      42,
		"enabled":    true,
		"file_paths": []string{"file1.go", "file2.go"},
	})

	// Retrieve with type safety
	projectID, ok := agentcontext.GetTypedContextParams[string](ctx, "project_id")
	if ok {
		fmt.Printf("Project: %s\n", projectID)
	}

	count, ok := agentcontext.GetTypedContextParams[int](ctx, "count")
	if ok {
		fmt.Printf("Count: %d\n", count)
	}

	enabled, ok := agentcontext.GetTypedContextParams[bool](ctx, "enabled")
	if ok {
		fmt.Printf("Enabled: %t\n", enabled)
	}

	filePaths, ok := agentcontext.GetTypedContextParams[[]string](ctx, "file_paths")
	if ok {
		fmt.Printf("Files: %v\n", filePaths)
	}

	// Output:
	// Project: project-123
	// Count: 42
	// Enabled: true
	// Files: [file1.go file2.go]
}

// ExampleGetAllContextParams demonstrates retrieving all parameters
func ExampleGetAllContextParams() {
	ctx := context.Background()
	ctx = agentcontext.InitContextParams(ctx)

	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": "project-123",
		"user_id":    "user-456",
	})

	allParams := agentcontext.GetAllContextParams(ctx)
	fmt.Printf("Total parameters: %d\n", len(allParams))
	// Output: Total parameters: 2
}

// ExampleContextManager_multiAgent demonstrates context usage across multiple agents
func ExampleContextManager_multiAgent() {
	// Initialize context at the start of the workflow
	ctx := context.Background()
	ctx = agentcontext.InitContextParams(ctx)

	// Chat Handler: Store project context
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": "project-123",
		"work_dir":   "/path/to/project",
	})

	// Intent Classifier: Read project context
	projectID, _ := agentcontext.GetTypedContextParams[string](ctx, "project_id")
	fmt.Printf("Classifying intent for project: %s\n", projectID)

	// Planner: Add plan information
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"plan_id": "plan-789",
	})

	// Executor: Read all context
	workDir, _ := agentcontext.GetTypedContextParams[string](ctx, "work_dir")
	planID, _ := agentcontext.GetTypedContextParams[string](ctx, "plan_id")
	fmt.Printf("Executing in: %s with plan: %s\n", workDir, planID)

	// Output:
	// Classifying intent for project: project-123
	// Executing in: /path/to/project with plan: plan-789
}
