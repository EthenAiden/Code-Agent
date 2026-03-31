package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_Create(t *testing.T) {
	authMiddleware := NewAuthMiddleware()
	assert.NotNil(t, authMiddleware)
}

func TestAuthMiddleware_Middleware(t *testing.T) {
	authMiddleware := NewAuthMiddleware()
	middleware := authMiddleware.Middleware()
	assert.NotNil(t, middleware)
}

func TestPanicRecoveryMiddleware_Create(t *testing.T) {
	middleware := PanicRecoveryMiddleware()
	assert.NotNil(t, middleware)
}

func TestIsValidUUID(t *testing.T) {
	validUUIDs := []string{
		"123e4567-e89b-12d3-a456-426614174000",
		"00000000-0000-4000-8000-000000000000",
		"ffffffff-ffff-4fff-bfff-ffffffffffff",
	}

	for _, uuid := range validUUIDs {
		assert.True(t, isValidUUID(uuid), "Expected %s to be valid", uuid)
	}

	invalidUUIDs := []string{
		"",
		"invalid",
		"123e4567-e89b-12d3-a456-42661417400",
		"123e4567-e89b-12d3-a456-4266141740000",
	}

	for _, uuid := range invalidUUIDs {
		assert.False(t, isValidUUID(uuid), "Expected %s to be invalid", uuid)
	}
}
