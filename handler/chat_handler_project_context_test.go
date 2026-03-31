package handler

import (
	"context"
	"testing"
	"time"

	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProjectManager is a mock implementation of tools.ProjectManagerInterface
type MockProjectManager struct {
	mock.Mock
}

func (m *MockProjectManager) GetSession(ctx context.Context, conversationID string, userID string) (*model.SessionDetails, error) {
	args := m.Called(ctx, conversationID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.SessionDetails), args.Error(1)
}

func (m *MockProjectManager) CreateSession(ctx context.Context, userID string) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *MockProjectManager) ListSessions(ctx context.Context, userID string, limit, offset int) ([]model.SessionSummary, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.SessionSummary), args.Error(1)
}

func (m *MockProjectManager) DeleteSession(ctx context.Context, conversationID string, userID string) error {
	args := m.Called(ctx, conversationID, userID)
	return args.Error(0)
}

// Ensure MockProjectManager implements tools.ProjectManagerInterface
var _ tools.ProjectManagerInterface = (*MockProjectManager)(nil)

// TestLoadProjectContext tests the loadProjectContext method
// Validates: Requirement 11.2 - Load project context before generating code
func TestLoadProjectContext(t *testing.T) {
	// Create mock project manager
	mockProjectManager := new(MockProjectManager)

	// Setup expected session details
	now := time.Now()
	expectedDetails := &model.SessionDetails{
		ConversationID: "test-project-123",
		MessageCount:   5,
		CreatedAt:      now,
		LastMessageAt:  &now,
	}

	// Configure mock to return session details
	mockProjectManager.On("GetSession", mock.Anything, "test-project-123", "test-user-456").
		Return(expectedDetails, nil)

	// Create chat handler with mock
	handler := &ChatHandler{
		projectManager: mockProjectManager,
	}

	// Call loadProjectContext
	ctx := context.Background()
	projectContext, err := handler.loadProjectContext(ctx, "test-project-123", "test-user-456")

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, projectContext)
	assert.Equal(t, "test-project-123", projectContext.ProjectID)
	assert.Equal(t, "test-user-456", projectContext.UserID)
	assert.Equal(t, 5, projectContext.MessageCount)
	assert.NotEmpty(t, projectContext.CreatedAt)

	// Verify mock was called
	mockProjectManager.AssertExpectations(t)
}

// TestLoadProjectContext_Error tests error handling in loadProjectContext
// Validates: Requirement 11.2 - Handle errors gracefully when loading project context
func TestLoadProjectContext_Error(t *testing.T) {
	// Create mock project manager
	mockProjectManager := new(MockProjectManager)

	// Configure mock to return error
	mockProjectManager.On("GetSession", mock.Anything, "invalid-project", "test-user").
		Return(nil, service.NewAPIError("SESSION_NOT_FOUND", assert.AnError))

	// Create chat handler with mock
	handler := &ChatHandler{
		projectManager: mockProjectManager,
	}

	// Call loadProjectContext
	ctx := context.Background()
	projectContext, err := handler.loadProjectContext(ctx, "invalid-project", "test-user")

	// Verify error is returned
	assert.Error(t, err)
	assert.Nil(t, projectContext)
	assert.Contains(t, err.Error(), "failed to get session details")

	// Verify mock was called
	mockProjectManager.AssertExpectations(t)
}
