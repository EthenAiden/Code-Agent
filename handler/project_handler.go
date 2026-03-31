package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/service"
)

// ProjectHandler handles session management endpoints
type ProjectHandler struct {
	projectManager *service.ProjectManager
}

// NewProjectHandler creates a new ProjectHandler with dependency injection
func NewProjectHandler(projectManager *service.ProjectManager) *ProjectHandler {
	return &ProjectHandler{
		projectManager: projectManager,
	}
}

// CreateSession handles POST /api/v1/sessions
func (h *ProjectHandler) CreateSession(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Parse request body
	var req model.CreateSessionRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   fmt.Sprintf("Invalid request body: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Call session manager to create session
	conversationID, err := h.projectManager.CreateSession(ctx, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "SESSION_CREATION_FAILED",
				Message:   fmt.Sprintf("Failed to create session: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Return conversation_id in response
	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]string{
			"conversation_id": conversationID,
		},
	})
}

// ListSessions handles GET /api/v1/sessions
func (h *ProjectHandler) ListSessions(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Parse pagination parameters
	limit := 20 // default
	offset := 0 // default

	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetParam := c.Query("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	// Call session manager to list sessions
	summaries, err := h.projectManager.ListSessions(ctx, userIDStr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "SESSION_LIST_FAILED",
				Message:   fmt.Sprintf("Failed to list sessions: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Calculate total from summaries (we need to get total count separately)
	// For now, we'll return the length as total (this should be fixed to get actual total)
	total := len(summaries)

	// Convert SessionSummary to ProjectSummary (they're compatible)
	projectSummaries := make([]model.ProjectSummary, len(summaries))
	for i, s := range summaries {
		projectSummaries[i] = model.ProjectSummary{
			ProjectID:            s.ConversationID,
			Name:                 "", // Will be populated from first message
			Description:          "",
			Icon:                 "💬",
			Thumbnail:            "bg-gradient-to-br from-gray-100 to-gray-200",
			LastMessageTimestamp: s.LastMessageTimestamp,
			MessageCount:         s.MessageCount,
		}
	}

	// Return paginated response
	c.JSON(http.StatusOK, model.APIResponse{
		Data: &model.ListProjectsResponse{
			Items:    projectSummaries,
			Total:    total,
			Page:     (offset / limit) + 1,
			PageSize: limit,
		},
	})
}

// GetSession handles GET /api/v1/projects/{project_id}
func (h *ProjectHandler) GetSession(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Extract project_id from path
	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "project_id path parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Call session manager to get session
	details, err := h.projectManager.GetSession(ctx, projectID, userIDStr)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, model.APIResponse{
				Error: &model.APIError{
					ErrorCode: "PROJECT_NOT_FOUND",
					Message:   fmt.Sprintf("Project with ID '%s' not found", projectID),
					Timestamp: time.Now().Format(time.RFC3339),
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "PROJECT_GET_FAILED",
				Message:   fmt.Sprintf("Failed to get project: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Return session details
	c.JSON(http.StatusOK, model.APIResponse{
		Data: details,
	})
}

// DeleteSession handles DELETE /api/v1/projects/{project_id}
func (h *ProjectHandler) DeleteSession(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Extract project_id from path
	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "project_id path parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Call session manager to delete session
	err := h.projectManager.DeleteSession(ctx, projectID, userIDStr)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(http.StatusNotFound, model.APIResponse{
				Error: &model.APIError{
					ErrorCode: "PROJECT_NOT_FOUND",
					Message:   fmt.Sprintf("Project with ID '%s' not found", projectID),
					Timestamp: time.Now().Format(time.RFC3339),
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "PROJECT_DELETE_FAILED",
				Message:   fmt.Sprintf("Failed to delete project: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Return success
	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]string{
			"message": "Project deleted successfully",
		},
	})
}
