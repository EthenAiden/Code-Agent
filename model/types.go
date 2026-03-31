package model

import (
	"time"
)

// Project represents a project (formerly session)
type Project struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Icon        string    `json:"icon" db:"icon"`
	Thumbnail   string    `json:"thumbnail" db:"thumbnail"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Version     int       `json:"version" db:"version"` // For optimistic locking
}

// Session is kept for backward compatibility, maps to Project
type Session = Project

// Message represents a single message in a project
type Message struct {
	ID             int       `json:"id" db:"id"`
	ConversationID string    `json:"conversation_id" db:"conversation_id"` // Actually project_id
	Role           string    `json:"role" db:"role"`                       // "user" or "assistant"
	Content        string    `json:"content" db:"content"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
	MessageIndex   int       `json:"message_index" db:"message_index"`
	Status         string    `json:"status" db:"status"` // "pending", "completed", "failed"
}

// ProjectSummary represents a summary of a project for listing
type ProjectSummary struct {
	ProjectID            string    `json:"project_id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	Icon                 string    `json:"icon"`
	Thumbnail            string    `json:"thumbnail"`
	LastMessageTimestamp time.Time `json:"last_message_timestamp"`
	MessageCount         int       `json:"message_count"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// SessionSummary is kept for backward compatibility
type SessionSummary struct {
	ConversationID       string    `json:"conversation_id"`
	LastMessageTimestamp time.Time `json:"last_message_timestamp"`
	MessageCount         int       `json:"message_count"`
}

// SessionDetails represents detailed session information
type SessionDetails struct {
	ConversationID string     `json:"conversation_id"`
	MessageCount   int        `json:"message_count"`
	CreatedAt      time.Time  `json:"created_at"`
	LastMessageAt  *time.Time `json:"last_message_at,omitempty"` // Pointer to handle NULL values
}

// APIResponse is the standard response format
type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error *APIError   `json:"error,omitempty"`
}

// APIError represents an error response
type APIError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// ListProjectsResponse represents a paginated list of projects
type ListProjectsResponse struct {
	Items    []ProjectSummary `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ListSessionsResponse is kept for backward compatibility
type ListSessionsResponse = ListProjectsResponse

// CreateProjectRequest is the request body for creating a project
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`
}

// CreateSessionRequest is kept for backward compatibility
type CreateSessionRequest = CreateProjectRequest

// ListSessionsRequest represents pagination parameters
type ListSessionsRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ChatRequest represents the chat message request
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents the chat response
type ChatResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}
