package context

import (
	"context"
	"sync"
)

// ContextManager manages session-specific parameters throughout agent execution.
// It provides thread-safe storage for project ID, file paths, and custom parameters.
type ContextManager struct {
	params *sync.Map
}

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const (
	// contextParamsKey is the key used to store the ContextManager in context
	contextParamsKey contextKey = "context_params"
)

// NewContextManager creates a new ContextManager instance
func NewContextManager() *ContextManager {
	return &ContextManager{
		params: &sync.Map{},
	}
}

// InitContextParams initializes a new context with an empty ContextManager.
// This should be called at the start of each agent workflow to create a fresh context.
func InitContextParams(ctx context.Context) context.Context {
	manager := NewContextManager()
	return context.WithValue(ctx, contextParamsKey, manager)
}

// AppendContextParams adds or updates multiple key-value pairs in the context.
// This function is thread-safe and can be called concurrently from multiple agents.
// If the context doesn't have a ContextManager, this function does nothing.
func AppendContextParams(ctx context.Context, values map[string]interface{}) {
	manager, ok := ctx.Value(contextParamsKey).(*ContextManager)
	if !ok {
		return
	}

	for key, value := range values {
		manager.params.Store(key, value)
	}
}

// GetTypedContextParams retrieves a typed value from the context parameters.
// It returns the value and a boolean indicating whether the key exists and the type matches.
// This function is thread-safe and can be called concurrently.
//
// Example usage:
//
//	projectID, ok := GetTypedContextParams[string](ctx, "project_id")
//	if ok {
//	    // use projectID
//	}
func GetTypedContextParams[T any](ctx context.Context, key string) (T, bool) {
	var zero T

	manager, ok := ctx.Value(contextParamsKey).(*ContextManager)
	if !ok {
		return zero, false
	}

	value, exists := manager.params.Load(key)
	if !exists {
		return zero, false
	}

	typedValue, ok := value.(T)
	if !ok {
		return zero, false
	}

	return typedValue, true
}

// GetContextParams retrieves an untyped value from the context parameters.
// It returns the value and a boolean indicating whether the key exists.
// This function is thread-safe and can be called concurrently.
func GetContextParams(ctx context.Context, key string) (interface{}, bool) {
	manager, ok := ctx.Value(contextParamsKey).(*ContextManager)
	if !ok {
		return nil, false
	}

	return manager.params.Load(key)
}

// DeleteContextParams removes a key from the context parameters.
// This function is thread-safe and can be called concurrently.
func DeleteContextParams(ctx context.Context, key string) {
	manager, ok := ctx.Value(contextParamsKey).(*ContextManager)
	if !ok {
		return
	}

	manager.params.Delete(key)
}

// GetAllContextParams returns a snapshot of all context parameters as a map.
// This is useful for debugging or serialization purposes.
// Note: This creates a copy, so modifications to the returned map won't affect the context.
func GetAllContextParams(ctx context.Context) map[string]interface{} {
	manager, ok := ctx.Value(contextParamsKey).(*ContextManager)
	if !ok {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{})
	manager.params.Range(func(key, value interface{}) bool {
		if strKey, ok := key.(string); ok {
			result[strKey] = value
		}
		return true
	})

	return result
}
