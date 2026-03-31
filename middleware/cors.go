package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// CORSMiddleware handles CORS (Cross-Origin Resource Sharing) headers
type CORSMiddleware struct{}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware() *CORSMiddleware {
	return &CORSMiddleware{}
}

// Middleware returns the CORS middleware handler
func (m *CORSMiddleware) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// Set CORS headers
		c.Header("Access-Control-Allow-Origin", "*") // Allow all origins (can be restricted to specific domains)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-User-ID, Authorization, Accept, Origin")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight OPTIONS request
		if string(c.Method()) == "OPTIONS" {
			c.AbortWithStatus(204) // No Content
			return
		}

		// Continue to next handler
		c.Next(ctx)
	}
}
