package service

import (
	"context"
	"fmt"

	"github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/repository"
)

// MessageHistoryService handles message history operations
type MessageHistoryService struct {
	sessionRepo *repository.MySQLSessionPersistenceRepository
	cacheRepo   *repository.CacheRepositoryImpl
}

// NewMessageHistoryService creates a new MessageHistoryService instance
func NewMessageHistoryService(sessionRepo *repository.MySQLSessionPersistenceRepository, cacheRepo *repository.CacheRepositoryImpl) *MessageHistoryService {
	return &MessageHistoryService{
		sessionRepo: sessionRepo,
		cacheRepo:   cacheRepo,
	}
}

// GetMessageHistory retrieves messages for a session from cache or MySQL
func (mhs *MessageHistoryService) GetMessageHistory(ctx context.Context, conversationID string, userID string) ([]model.Message, error) {
	// Try to get from cache first
	cachedMessages, err := mhs.cacheRepo.GetSessionCache(ctx, conversationID, userID)
	if err != nil {
		// Redis unavailable - fallback to MySQL
		if apiErr, ok := err.(*repository.RepositoryError); ok && apiErr.ErrorCode == "REDIS_UNAVAILABLE" {
			fmt.Printf("Warning: Redis unavailable, falling back to MySQL: %v\n", apiErr.Err)
			return mhs.getMessageHistoryFromMySQL(ctx, conversationID, userID)
		}
		return nil, ConvertRepositoryError("MESSAGE_HISTORY_FAILED", err)
	}

	// Check if cache hit (non-empty messages)
	if len(cachedMessages) > 0 {
		return cachedMessages, nil
	}

	// Cache miss or empty - need to reconstruct from MySQL
	// Acquire lock to prevent cache breakdown
	err = mhs.cacheRepo.CacheLock(ctx, conversationID, userID)
	if err != nil {
		// Lock acquisition failed - fallback to MySQL
		return mhs.getMessageHistoryFromMySQL(ctx, conversationID, userID)
	}
	defer mhs.cacheRepo.CacheUnlock(ctx, conversationID, userID)

	// Double-check cache after acquiring lock
	cachedMessages, err = mhs.cacheRepo.GetSessionCache(ctx, conversationID, userID)
	if err == nil && len(cachedMessages) > 0 {
		return cachedMessages, nil
	}

	// Fetch from MySQL
	return mhs.getMessageHistoryFromMySQLAndCache(ctx, conversationID, userID)
}

// getMessageHistoryFromMySQL fetches messages directly from MySQL
func (mhs *MessageHistoryService) getMessageHistoryFromMySQL(ctx context.Context, conversationID string, userID string) ([]model.Message, error) {
	messages, err := mhs.sessionRepo.GetSessionMessages(ctx, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages from MySQL: %w", err)
	}

	return messages, nil
}

// getMessageHistoryFromMySQLAndCache fetches from MySQL and populates cache
func (mhs *MessageHistoryService) getMessageHistoryFromMySQLAndCache(ctx context.Context, conversationID string, userID string) ([]model.Message, error) {
	messages, err := mhs.sessionRepo.GetSessionMessages(ctx, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages from MySQL: %w", err)
	}

	// Cache the messages with randomized TTL
	ttl := GenerateRandomTTL()
	err = mhs.cacheRepo.SetSessionCache(ctx, conversationID, userID, messages, ttl)
	if err != nil {
		// Log warning but don't fail - data was successfully retrieved from MySQL
		fmt.Printf("Warning: failed to set cache entry: %v\n", err)
	}

	return messages, nil
}

// CreateSessionIfNotExists creates a session if it doesn't exist
func (mhs *MessageHistoryService) CreateSessionIfNotExists(ctx context.Context, conversationID string, userID string) error {
	// Check if session exists first
	exists, err := mhs.sessionRepo.SessionExists(ctx, conversationID, userID)
	if err != nil {
		return ConvertRepositoryError("SESSION_CREATION_FAILED", err)
	}

	if exists {
		// Session already exists, nothing to do
		return nil
	}

	// Create session in MySQL
	err = mhs.sessionRepo.InsertSession(ctx, conversationID, userID)
	if err != nil {
		return ConvertRepositoryError("SESSION_CREATION_FAILED", err)
	}

	// Create cache entry with randomized TTL
	ttl := GenerateRandomTTL()
	err = mhs.cacheRepo.SetSessionCache(ctx, conversationID, userID, []model.Message{}, ttl)
	if err != nil {
		// Log warning but don't fail - session is already created in MySQL
		if apiErr, ok := err.(*repository.RepositoryError); ok && apiErr.ErrorCode == "REDIS_UNAVAILABLE" {
			fmt.Printf("Warning: Redis unavailable, session created in MySQL only: %v\n", apiErr.Err)
			return nil
		}
		return ConvertRepositoryError("SESSION_CREATION_FAILED", err)
	}

	return nil
}

// InsertMessage stores a message in MySQL with status tracking
func (mhs *MessageHistoryService) InsertMessage(ctx context.Context, msg model.Message, userID string) error {
	// Validate message_index uniqueness before insertion
	nextIndex, err := mhs.sessionRepo.GetNextMessageIndex(ctx, msg.ConversationID, userID)
	if err != nil {
		return ConvertRepositoryError("MESSAGE_INSERT_FAILED", err)
	}

	// Use the calculated index
	msg.MessageIndex = nextIndex

	// Store message with status=pending
	msg.Status = "pending"
	err = mhs.sessionRepo.InsertMessage(ctx, msg)
	if err != nil {
		return ConvertRepositoryError("MESSAGE_INSERT_FAILED", err)
	}

	// Invalidate cache after inserting message to ensure consistency
	// This ensures the next GetMessageHistory call will fetch fresh data from MySQL
	err = mhs.cacheRepo.DeleteSessionCache(ctx, msg.ConversationID, userID)
	if err != nil {
		// Log warning but don't fail - message was successfully inserted
		if apiErr, ok := err.(*repository.RepositoryError); ok && apiErr.ErrorCode == "REDIS_UNAVAILABLE" {
			fmt.Printf("Warning: Redis unavailable, cache invalidation skipped: %v\n", apiErr.Err)
		} else {
			fmt.Printf("Warning: failed to invalidate cache: %v\n", err)
		}
	}

	return nil
}

// UpdateMessageStatus updates the status of a message
func (mhs *MessageHistoryService) UpdateMessageStatus(ctx context.Context, conversationID string, messageIndex int, status string) error {
	err := mhs.sessionRepo.UpdateMessageStatus(ctx, conversationID, messageIndex, status)
	if err != nil {
		return ConvertRepositoryError("MESSAGE_UPDATE_FAILED", err)
	}

	return nil
}

// CleanupFailedMessages removes failed messages from a session
func (mhs *MessageHistoryService) CleanupFailedMessages(ctx context.Context, conversationID string, userID string) error {
	err := mhs.sessionRepo.CleanupFailedMessages(ctx, conversationID, userID)
	if err != nil {
		return ConvertRepositoryError("MESSAGE_CLEANUP_FAILED", err)
	}

	// Clear cache after cleanup
	err = mhs.cacheRepo.DeleteSessionCache(ctx, conversationID, userID)
	if err != nil {
		// Log warning but don't fail
		if apiErr, ok := err.(*repository.RepositoryError); ok && apiErr.ErrorCode == "REDIS_UNAVAILABLE" {
			fmt.Printf("Warning: Redis unavailable, cache cleanup skipped: %v\n", apiErr.Err)
			return nil
		}
		return ConvertRepositoryError("MESSAGE_CLEANUP_FAILED", err)
	}

	return nil
}

// GetLastMessageIndex gets the last message index for a session
func (mhs *MessageHistoryService) GetLastMessageIndex(ctx context.Context, conversationID string, userID string) (int, error) {
	return mhs.sessionRepo.GetNextMessageIndex(ctx, conversationID, userID)
}
