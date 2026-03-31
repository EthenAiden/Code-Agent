package repository

import (
	"testing"
	"time"

	"github.com/ethen-aiden/code-agent/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestCacheRepositoryImpl_CacheKeyWithUser(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	cr := NewCacheRepository(redisClient, 24*time.Hour, 60*time.Second)

	conversationID := "123e4567-e89b-12d3-a456-426614174000"
	userID := "123e4567-e89b-12d3-a456-426614174001"

	key := cr.CacheKeyWithUser(conversationID, userID)
	expectedKey := "session:123e4567-e89b-12d3-a456-426614174000:123e4567-e89b-12d3-a456-426614174001"

	assert.Equal(t, expectedKey, key)
}

func TestCacheRepositoryImpl_IsCacheMiss(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	cr := NewCacheRepository(redisClient, 24*time.Hour, 60*time.Second)

	// Test cache miss with error
	assert.True(t, cr.IsCacheMiss(nil, redis.Nil))

	// Test cache miss with empty array
	assert.True(t, cr.IsCacheMiss([]model.Message{}, nil))

	// Test cache hit with non-empty array
	messages := []model.Message{
		{ConversationID: "123e4567-e89b-12d3-a456-426614174000", Role: "user", Content: "Hello", Timestamp: time.Now(), MessageIndex: 0, Status: "pending"},
	}
	assert.False(t, cr.IsCacheMiss(messages, nil))
}
