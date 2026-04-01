package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/repository"
	"github.com/google/uuid"
	"golang.org/x/exp/rand"
)

// APIError represents an API error with error code
type APIError struct {
	ErrorCode string
	Err       error
}

// Error returns the error message
func (e *APIError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a new API error
func NewAPIError(errorCode string, err error) *APIError {
	return &APIError{
		ErrorCode: errorCode,
		Err:       err,
	}
}

// ConvertRepositoryError converts a repository error to an API error
func ConvertRepositoryError(errorCode string, err error) *APIError {
	if repoErr, ok := err.(*repository.RepositoryError); ok {
		// Map Redis errors to appropriate API errors
		if repoErr.ErrorCode == "REDIS_UNAVAILABLE" {
			// For Redis errors, we fallback to MySQL so don't return error to client
			// Log the error but continue with MySQL operations
			fmt.Printf("Warning: Redis unavailable, falling back to MySQL: %v\n", repoErr.Err)
			return nil
		}
		return NewAPIError(errorCode, repoErr)
	}
	return NewAPIError(errorCode, err)
}

// ToAPIError converts an error to an API response error
func ToAPIError(err error) *model.APIError {
	if apiErr, ok := err.(*APIError); ok {
		return &model.APIError{
			ErrorCode: apiErr.ErrorCode,
			Message:   apiErr.Error(),
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}
	return &model.APIError{
		ErrorCode: "INTERNAL_SERVER_ERROR",
		Message:   err.Error(),
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// ProjectManager handles session lifecycle operations
type ProjectManager struct {
	sessionRepo *repository.MySQLSessionPersistenceRepository
	cacheRepo   *repository.CacheRepositoryImpl
}

// NewProjectManager creates a new ProjectManager instance
func NewProjectManager(sessionRepo *repository.MySQLSessionPersistenceRepository, cacheRepo *repository.CacheRepositoryImpl) *ProjectManager {
	return &ProjectManager{
		sessionRepo: sessionRepo,
		cacheRepo:   cacheRepo,
	}
}

// GenerateRandomTTL generates a random TTL with ±10% variation from base 24 hours
func GenerateRandomTTL() time.Duration {
	baseTTL := 24 * time.Hour
	variation := float64(baseTTL) * 0.1 // 10% variation
	minTTL := float64(baseTTL) - variation
	maxTTL := float64(baseTTL) + variation

	// Generate random value in range
	randValue := rand.Float64()*(maxTTL-minTTL) + minTTL
	return time.Duration(randValue)
}

// CreateSession creates a new session with UUID v4 and cache entry
func (sm *ProjectManager) CreateSession(ctx context.Context, userID string) (string, error) {
	// Generate UUID v4 for conversation_id
	conversationID, err := uuid.NewRandom()
	if err != nil {
		return "", NewAPIError("UUID_GENERATION_FAILED", fmt.Errorf("failed to generate UUID: %w", err))
	}

	conversationIDStr := conversationID.String()

	// Check if session already exists (idempotent operation)
	exists, err := sm.sessionRepo.SessionExists(ctx, conversationIDStr, userID)
	if err != nil {
		return "", ConvertRepositoryError("SESSION_CREATION_FAILED", err)
	}

	if exists {
		// Session already exists, return the existing ID
		return conversationIDStr, nil
	}

	// Create session in MySQL
	err = sm.sessionRepo.InsertSession(ctx, conversationIDStr, userID)
	if err != nil {
		return "", ConvertRepositoryError("SESSION_CREATION_FAILED", err)
	}

	// Create cache entry with randomized TTL
	ttl := GenerateRandomTTL()
	err = sm.cacheRepo.SetSessionCache(ctx, conversationIDStr, userID, []model.Message{}, ttl)
	if err != nil {
		// Log warning but don't fail - session is already created in MySQL
		fmt.Printf("Warning: failed to set cache entry: %v\n", err)
	}

	return conversationIDStr, nil
}

// ListSessions retrieves paginated session summaries for a user
func (sm *ProjectManager) ListSessions(ctx context.Context, userID string, limit, offset int) ([]model.SessionSummary, error) {
	// Note: For list operations, we'll use MySQL directly as caching paginated results
	// is more complex and the task specifies fallback to MySQL when cache unavailable
	summaries, err := sm.sessionRepo.ListSessions(ctx, userID, limit, offset)
	if err != nil {
		return nil, ConvertRepositoryError("SESSION_LIST_FAILED", err)
	}

	return summaries, nil
}

// GetSession retrieves detailed session information
func (sm *ProjectManager) GetSession(ctx context.Context, conversationID string, userID string) (*model.SessionDetails, error) {
	// Check if session exists and belongs to user
	exists, err := sm.sessionRepo.SessionExists(ctx, conversationID, userID)
	if err != nil {
		return nil, ConvertRepositoryError("SESSION_GET_FAILED", err)
	}

	if !exists {
		return nil, NewAPIError("SESSION_NOT_FOUND", fmt.Errorf("session not found"))
	}

	// Get session details
	details, err := sm.sessionRepo.GetSessionDetails(ctx, conversationID, userID)
	if err != nil {
		return nil, ConvertRepositoryError("SESSION_GET_FAILED", err)
	}

	return details, nil
}

// DeleteSession removes a session and all its data
func (sm *ProjectManager) DeleteSession(ctx context.Context, conversationID string, userID string) error {
	// Check if session exists and belongs to user
	exists, err := sm.sessionRepo.SessionExists(ctx, conversationID, userID)
	if err != nil {
		return ConvertRepositoryError("SESSION_DELETE_FAILED", err)
	}

	if !exists {
		return NewAPIError("SESSION_NOT_FOUND", fmt.Errorf("session not found"))
	}

	// Delete from MySQL
	err = sm.sessionRepo.DeleteSession(ctx, conversationID, userID)
	if err != nil {
		return ConvertRepositoryError("SESSION_DELETE_FAILED", err)
	}

	// Delete from Redis cache
	err = sm.cacheRepo.DeleteSessionCache(ctx, conversationID, userID)
	if err != nil {
		// Log warning but don't fail - session is already deleted from MySQL
		fmt.Printf("Warning: failed to delete cache entry: %v\n", err)
	}

	return nil
}

// SetFramework stores the chosen framework for a project and invalidates the cache.
func (sm *ProjectManager) SetFramework(ctx context.Context, conversationID string, userID string, framework string) error {
	err := sm.sessionRepo.SetFramework(ctx, conversationID, userID, framework)
	if err != nil {
		return ConvertRepositoryError("FRAMEWORK_UPDATE_FAILED", err)
	}
	// Invalidate Redis cache so next GetSession reflects the new framework
	if sm.cacheRepo != nil {
		_ = sm.cacheRepo.DeleteSessionCache(ctx, conversationID, userID)
	}
	return nil
}
