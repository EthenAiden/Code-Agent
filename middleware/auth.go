package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/ethen-aiden/code-agent/model"
)

// AuthMiddleware handles user authentication via X-User-ID header
type AuthMiddleware struct{}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// isValidUUID validates if a string is a valid UUID v4
func isValidUUID(s string) bool {
	// UUID v4 format: xxxxxxxx-xxxx-4xxx-[89ab]xxx-xxxxxxxxxxxx
	// The 13th character indicates version (4 for UUID v4)
	// The 17th character indicates variant (8, 9, a, or b for RFC 4122)
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	return uuidRegex.MatchString(s)
}

// Middleware returns the Hertz middleware function
func (m *AuthMiddleware) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// Extract X-User-ID header
		userID := string(c.GetHeader("X-User-ID"))

		// Check if header is missing
		if userID == "" {
			c.JSON(http.StatusUnauthorized, model.APIResponse{
				Error: &model.APIError{
					ErrorCode: "USER_ID_MISSING",
					Message:   "X-User-ID header is required",
					Timestamp: time.Now().Format(time.RFC3339),
				},
			})
			c.Abort()
			return
		}

		// Validate UUID v4 format
		if !isValidUUID(userID) {
			c.JSON(http.StatusUnauthorized, model.APIResponse{
				Error: &model.APIError{
					ErrorCode: "USER_ID_INVALID",
					Message:   "X-User-ID must be a valid UUID v4",
					Timestamp: time.Now().Format(time.RFC3339),
				},
			})
			c.Abort()
			return
		}

		// Store user_id in context for downstream handlers
		c.Set("user_id", userID)
		c.Next(ctx)
	}
}

// PanicRecoveryMiddleware recovers from panics and converts to error responses
func PanicRecoveryMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				fmt.Printf("Panic recovered: %v\n", err)

				// Return consistent error response
				c.JSON(http.StatusInternalServerError, model.APIResponse{
					Error: &model.APIError{
						ErrorCode: "INTERNAL_SERVER_ERROR",
						Message:   "An internal server error occurred",
						Timestamp: time.Now().Format(time.RFC3339),
					},
				})
			}
		}()

		c.Next(ctx)
	}
}
