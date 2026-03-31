package context

import (
	"context"
	"sync"
	"testing"
)

func TestInitContextParams(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	// Verify that a ContextManager was added to the context
	manager, ok := ctx.Value(contextParamsKey).(*ContextManager)
	if !ok {
		t.Fatal("InitContextParams did not add ContextManager to context")
	}

	if manager == nil {
		t.Fatal("ContextManager is nil")
	}

	if manager.params == nil {
		t.Fatal("ContextManager.params is nil")
	}
}

func TestAppendContextParams(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	// Test adding single parameter
	AppendContextParams(ctx, map[string]interface{}{
		"project_id": "test-project-123",
	})

	value, ok := GetContextParams(ctx, "project_id")
	if !ok {
		t.Fatal("Failed to retrieve project_id")
	}

	if value != "test-project-123" {
		t.Errorf("Expected 'test-project-123', got '%v'", value)
	}

	// Test adding multiple parameters
	AppendContextParams(ctx, map[string]interface{}{
		"user_id":  "user-456",
		"work_dir": "/path/to/project",
	})

	userID, ok := GetContextParams(ctx, "user_id")
	if !ok || userID != "user-456" {
		t.Errorf("Failed to retrieve user_id correctly")
	}

	workDir, ok := GetContextParams(ctx, "work_dir")
	if !ok || workDir != "/path/to/project" {
		t.Errorf("Failed to retrieve work_dir correctly")
	}

	// Test updating existing parameter
	AppendContextParams(ctx, map[string]interface{}{
		"project_id": "updated-project-789",
	})

	value, ok = GetContextParams(ctx, "project_id")
	if !ok || value != "updated-project-789" {
		t.Errorf("Failed to update project_id")
	}
}

func TestAppendContextParamsWithoutInit(t *testing.T) {
	ctx := context.Background()

	// Should not panic when context doesn't have ContextManager
	AppendContextParams(ctx, map[string]interface{}{
		"project_id": "test-project",
	})

	// Verify nothing was stored
	_, ok := GetContextParams(ctx, "project_id")
	if ok {
		t.Error("GetContextParams should return false for uninitialized context")
	}
}

func TestGetTypedContextParams(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	// Store different types
	AppendContextParams(ctx, map[string]interface{}{
		"project_id": "test-project",
		"count":      42,
		"enabled":    true,
		"file_paths": []string{"file1.go", "file2.go"},
		"metadata":   map[string]string{"key": "value"},
	})

	// Test string retrieval
	projectID, ok := GetTypedContextParams[string](ctx, "project_id")
	if !ok {
		t.Fatal("Failed to retrieve project_id as string")
	}
	if projectID != "test-project" {
		t.Errorf("Expected 'test-project', got '%s'", projectID)
	}

	// Test int retrieval
	count, ok := GetTypedContextParams[int](ctx, "count")
	if !ok {
		t.Fatal("Failed to retrieve count as int")
	}
	if count != 42 {
		t.Errorf("Expected 42, got %d", count)
	}

	// Test bool retrieval
	enabled, ok := GetTypedContextParams[bool](ctx, "enabled")
	if !ok {
		t.Fatal("Failed to retrieve enabled as bool")
	}
	if !enabled {
		t.Error("Expected true, got false")
	}

	// Test slice retrieval
	filePaths, ok := GetTypedContextParams[[]string](ctx, "file_paths")
	if !ok {
		t.Fatal("Failed to retrieve file_paths as []string")
	}
	if len(filePaths) != 2 || filePaths[0] != "file1.go" {
		t.Errorf("Unexpected file_paths: %v", filePaths)
	}

	// Test map retrieval
	metadata, ok := GetTypedContextParams[map[string]string](ctx, "metadata")
	if !ok {
		t.Fatal("Failed to retrieve metadata as map[string]string")
	}
	if metadata["key"] != "value" {
		t.Errorf("Unexpected metadata: %v", metadata)
	}

	// Test type mismatch
	_, ok = GetTypedContextParams[int](ctx, "project_id")
	if ok {
		t.Error("Should return false when type doesn't match")
	}

	// Test non-existent key
	_, ok = GetTypedContextParams[string](ctx, "non_existent")
	if ok {
		t.Error("Should return false for non-existent key")
	}
}

func TestGetTypedContextParamsWithoutInit(t *testing.T) {
	ctx := context.Background()

	// Should return false when context doesn't have ContextManager
	_, ok := GetTypedContextParams[string](ctx, "project_id")
	if ok {
		t.Error("Should return false for uninitialized context")
	}
}

func TestDeleteContextParams(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	// Add parameters
	AppendContextParams(ctx, map[string]interface{}{
		"project_id": "test-project",
		"user_id":    "test-user",
	})

	// Verify they exist
	_, ok := GetContextParams(ctx, "project_id")
	if !ok {
		t.Fatal("project_id should exist")
	}

	// Delete one parameter
	DeleteContextParams(ctx, "project_id")

	// Verify it's deleted
	_, ok = GetContextParams(ctx, "project_id")
	if ok {
		t.Error("project_id should be deleted")
	}

	// Verify other parameter still exists
	_, ok = GetContextParams(ctx, "user_id")
	if !ok {
		t.Error("user_id should still exist")
	}

	// Delete non-existent key (should not panic)
	DeleteContextParams(ctx, "non_existent")
}

func TestDeleteContextParamsWithoutInit(t *testing.T) {
	ctx := context.Background()

	// Should not panic when context doesn't have ContextManager
	DeleteContextParams(ctx, "project_id")
}

func TestGetAllContextParams(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	// Add multiple parameters
	params := map[string]interface{}{
		"project_id": "test-project",
		"user_id":    "test-user",
		"count":      42,
	}
	AppendContextParams(ctx, params)

	// Get all parameters
	allParams := GetAllContextParams(ctx)

	if len(allParams) != 3 {
		t.Errorf("Expected 3 parameters, got %d", len(allParams))
	}

	if allParams["project_id"] != "test-project" {
		t.Error("project_id mismatch")
	}

	if allParams["user_id"] != "test-user" {
		t.Error("user_id mismatch")
	}

	if allParams["count"] != 42 {
		t.Error("count mismatch")
	}

	// Verify modifying returned map doesn't affect context
	allParams["new_key"] = "new_value"
	_, ok := GetContextParams(ctx, "new_key")
	if ok {
		t.Error("Modifying returned map should not affect context")
	}
}

func TestGetAllContextParamsWithoutInit(t *testing.T) {
	ctx := context.Background()

	// Should return empty map when context doesn't have ContextManager
	allParams := GetAllContextParams(ctx)
	if len(allParams) != 0 {
		t.Error("Should return empty map for uninitialized context")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				AppendContextParams(ctx, map[string]interface{}{
					"key": id*numOperations + j,
				})
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				GetContextParams(ctx, "key")
				GetTypedContextParams[int](ctx, "key")
			}
		}()
	}

	// Concurrent deletes
	for i := 0; i < numGoroutines/10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				DeleteContextParams(ctx, "key")
			}
		}()
	}

	wg.Wait()

	// Test should complete without panics or race conditions
}

func TestContextIsolation(t *testing.T) {
	// Create two separate contexts
	ctx1 := InitContextParams(context.Background())
	ctx2 := InitContextParams(context.Background())

	// Add different parameters to each
	AppendContextParams(ctx1, map[string]interface{}{
		"project_id": "project-1",
	})

	AppendContextParams(ctx2, map[string]interface{}{
		"project_id": "project-2",
	})

	// Verify isolation
	projectID1, ok := GetTypedContextParams[string](ctx1, "project_id")
	if !ok || projectID1 != "project-1" {
		t.Error("Context 1 should have project-1")
	}

	projectID2, ok := GetTypedContextParams[string](ctx2, "project_id")
	if !ok || projectID2 != "project-2" {
		t.Error("Context 2 should have project-2")
	}
}

func TestComplexDataTypes(t *testing.T) {
	ctx := context.Background()
	ctx = InitContextParams(ctx)

	// Test with complex nested structures
	type ProjectMetadata struct {
		Name         string
		Version      string
		Dependencies []string
		Config       map[string]interface{}
	}

	metadata := ProjectMetadata{
		Name:         "test-project",
		Version:      "1.0.0",
		Dependencies: []string{"dep1", "dep2"},
		Config: map[string]interface{}{
			"debug":   true,
			"timeout": 30,
		},
	}

	AppendContextParams(ctx, map[string]interface{}{
		"project_metadata": metadata,
	})

	// Retrieve and verify
	retrieved, ok := GetTypedContextParams[ProjectMetadata](ctx, "project_metadata")
	if !ok {
		t.Fatal("Failed to retrieve ProjectMetadata")
	}

	if retrieved.Name != "test-project" {
		t.Errorf("Name mismatch: expected 'test-project', got '%s'", retrieved.Name)
	}

	if len(retrieved.Dependencies) != 2 {
		t.Errorf("Dependencies length mismatch")
	}

	if retrieved.Config["debug"] != true {
		t.Error("Config debug value mismatch")
	}
}
