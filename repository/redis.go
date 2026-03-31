package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ethen-aiden/code-agent/model"
	"github.com/redis/go-redis/v9"
)

// CacheRepositoryImpl implements CacheRepository interface using Redis
type CacheRepositoryImpl struct {
	redisClient *redis.Client
	sessionTTL  time.Duration
	emptyTTL    time.Duration
}

// NewCacheRepository creates a new Redis cache repository instance
func NewCacheRepository(redisClient *redis.Client, sessionTTL, emptyTTL time.Duration) *CacheRepositoryImpl {
	return &CacheRepositoryImpl{
		redisClient: redisClient,
		sessionTTL:  sessionTTL,
		emptyTTL:    emptyTTL,
	}
}

// GetSessionCache retrieves session messages from Redis cache
func (cr *CacheRepositoryImpl) GetSessionCache(ctx context.Context, conversationID string, userID string) ([]model.Message, error) {
	key := cr.CacheKeyWithUser(conversationID, userID)

	// Try to get from cache
	result, err := cr.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - return empty array to indicate session doesn't exist in cache
			return []model.Message{}, nil
		}
		// Redis connection error - return error for caller to handle
		return nil, NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("redis get failed: %w", err))
	}

	// Parse JSON data
	messages, err := UnmarshalMessages([]byte(result))
	if err != nil {
		return nil, NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("failed to unmarshal messages: %w", err))
	}

	return messages, nil
}

// SetSessionCache stores session messages in Redis with TTL
func (cr *CacheRepositoryImpl) SetSessionCache(ctx context.Context, conversationID string, userID string, messages []model.Message, ttl time.Duration) error {
	key := cr.CacheKeyWithUser(conversationID, userID)

	// Marshal messages to JSON
	data, err := MarshalMessages(messages)
	if err != nil {
		return NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("failed to marshal messages: %w", err))
	}

	// Set with TTL
	err = cr.redisClient.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("redis set failed: %w", err))
	}

	return nil
}

// DeleteSessionCache removes session from Redis cache
func (cr *CacheRepositoryImpl) DeleteSessionCache(ctx context.Context, conversationID string, userID string) error {
	key := cr.CacheKeyWithUser(conversationID, userID)

	err := cr.redisClient.Del(ctx, key).Err()
	if err != nil {
		return NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("redis del failed: %w", err))
	}

	return nil
}

// CacheKeyWithUser returns the cache key with user ID for proper isolation
func (cr *CacheRepositoryImpl) CacheKeyWithUser(conversationID string, userID string) string {
	return fmt.Sprintf("session:%s:%s", conversationID, userID)
}

// CacheLock acquires a mutex lock for cache reconstruction using SETNX
func (cr *CacheRepositoryImpl) CacheLock(ctx context.Context, conversationID string, userID string) error {
	key := fmt.Sprintf("session:%s:%s:lock", conversationID, userID)

	// Use SETNX with 30 second expiration
	result, err := cr.redisClient.SetNX(ctx, key, "1", 30*time.Second).Result()
	if err != nil {
		return NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("redis setnx failed: %w", err))
	}

	// If result is false, lock acquisition failed (another process holds the lock)
	if !result {
		return fmt.Errorf("cache lock acquisition failed: lock already held")
	}

	return nil
}

// CacheUnlock releases the mutex lock
func (cr *CacheRepositoryImpl) CacheUnlock(ctx context.Context, conversationID string, userID string) error {
	key := fmt.Sprintf("session:%s:%s:lock", conversationID, userID)

	err := cr.redisClient.Del(ctx, key).Err()
	if err != nil {
		return NewRepositoryError("REDIS_UNAVAILABLE", fmt.Errorf("redis del failed: %w", err))
	}

	return nil
}

// IsCacheMiss returns true if the key doesn't exist or is empty
func (cr *CacheRepositoryImpl) IsCacheMiss(data []model.Message, err error) bool {
	if err != nil {
		return true
	}
	return len(data) == 0
}
