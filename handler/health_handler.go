package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
}

// Health handles GET /health
func (h *HealthHandler) Health(ctx context.Context, c *app.RequestContext) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   "agent-server",
		Version:   "1.0.0",
	}

	c.JSON(http.StatusOK, response)
}

// ReadinessResponse represents the readiness check response
type ReadinessResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// Readiness handles GET /ready
// This endpoint can be extended to check database, Redis, and other dependencies
func (h *HealthHandler) Readiness(ctx context.Context, c *app.RequestContext) {
	checks := make(map[string]string)
	checks["server"] = "ok"
	// TODO: Add database check
	// TODO: Add Redis check

	response := ReadinessResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Checks:    checks,
	}

	c.JSON(http.StatusOK, response)
}
