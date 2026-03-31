package repository

import (
	"context"
	"time"

	"github.com/ethen-aiden/code-agent/model"
)

// SessionPersistenceRepository defines the interface for session persistence operations
type SessionPersistenceRepository interface {
	// InsertSession creates a new session in MySQL
	InsertSession(ctx context.Context, conversationID string, userID string) error

	// InsertMessage stores a message in MySQL
	InsertMessage(ctx context.Context, msg model.Message) error

	// GetSessionMessages retrieves all messages for a session
	GetSessionMessages(ctx context.Context, conversationID string, userID string) ([]model.Message, error)

	// ListSessions retrieves session summaries with pagination
	ListSessions(ctx context.Context, userID string, limit, offset int) ([]model.SessionSummary, error)

	// DeleteSession removes a session and all its messages
	DeleteSession(ctx context.Context, conversationID string, userID string) error

	// GetSessionDetails retrieves session metadata
	GetSessionDetails(ctx context.Context, conversationID string, userID string) (*model.SessionDetails, error)

	// SessionExists checks if a session exists
	SessionExists(ctx context.Context, conversationID string, userID string) (bool, error)

	// GetSessionVersion retrieves the current version for optimistic locking
	GetSessionVersion(ctx context.Context, conversationID string, userID string) (int, error)

	// UpdateSessionVersion updates the session version for optimistic locking
	UpdateSessionVersion(ctx context.Context, conversationID string, userID string, expectedVersion int) error

	// GetNextMessageIndex gets the next message index for a session
	GetNextMessageIndex(ctx context.Context, conversationID string, userID string) (int, error)

	// CleanupFailedMessages removes failed messages from a session
	CleanupFailedMessages(ctx context.Context, conversationID string, userID string) error

	// UpdateMessageStatus updates the status of a message
	UpdateMessageStatus(ctx context.Context, conversationID string, messageIndex int, status string) error
}

// CacheRepository defines the interface for Redis caching operations
type CacheRepository interface {
	// GetSessionCache retrieves session messages from Redis cache
	GetSessionCache(ctx context.Context, conversationID string, userID string) ([]model.Message, error)

	// SetSessionCache stores session messages in Redis with TTL
	SetSessionCache(ctx context.Context, conversationID string, userID string, messages []model.Message, ttl time.Duration) error

	// DeleteSessionCache removes session from Redis cache
	DeleteSessionCache(ctx context.Context, conversationID string, userID string) error

	// CacheKeyWithUser returns the cache key with user ID for proper isolation
	CacheKeyWithUser(conversationID string, userID string) string

	// CacheLock acquires a mutex lock for cache reconstruction
	CacheLock(ctx context.Context, conversationID string, userID string) error

	// CacheUnlock releases the mutex lock
	CacheUnlock(ctx context.Context, conversationID string, userID string) error

	// IsCacheMiss returns true if the key doesn't exist or is empty
	IsCacheMiss(data []model.Message, err error) bool
}
