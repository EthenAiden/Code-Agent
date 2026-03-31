package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethen-aiden/code-agent/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProjectManager is a mock implementation of ProjectManager
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

func TestGetProjectContextTool_Info(t *testing.T) {
	mockManager := new(MockProjectManager)
	tool := NewGetProjectContextTool(mockManager, "/test/root")

	info, err := tool.Info(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "get_project_context", info.Name)
	assert.Contains(t, info.Desc, "project context")
}

func TestGetProjectContextTool_InvokableRun_Success(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "project-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Test Project"), 0644)

	// Setup mock
	mockManager := new(MockProjectManager)
	lastMessageTime := time.Now()
	mockManager.On("GetSession", mock.Anything, "test-project-id", "test-user-id").Return(&model.SessionDetails{
		ConversationID: "test-project-id",
		MessageCount:   5,
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		LastMessageAt:  &lastMessageTime,
	}, nil)

	tool := NewGetProjectContextTool(mockManager, tempDir)

	// Prepare input
	input := &GetProjectContextInput{
		ProjectID:    "test-project-id",
		UserID:       "test-user-id",
		IncludeFiles: true,
		MaxDepth:     2,
	}
	inputJSON, _ := json.Marshal(input)

	// Execute tool
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	// Verify results
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse output
	var output ProjectContextOutput
	err = json.Unmarshal([]byte(result), &output)
	assert.NoError(t, err)
	assert.Equal(t, "test-project-id", output.ProjectID)
	assert.Equal(t, "test-user-id", output.UserID)
	assert.Equal(t, 5, output.MessageCount)
	assert.NotNil(t, output.FileStructure)
	assert.Contains(t, output.Summary, "test-project-id")

	mockManager.AssertExpectations(t)
}

func TestGetProjectContextTool_InvokableRun_WithoutFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "project-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	mockManager := new(MockProjectManager)
	lastMessageTime := time.Now()
	mockManager.On("GetSession", mock.Anything, "test-project-id", "test-user-id").Return(&model.SessionDetails{
		ConversationID: "test-project-id",
		MessageCount:   3,
		CreatedAt:      time.Now().Add(-12 * time.Hour),
		LastMessageAt:  &lastMessageTime,
	}, nil)

	tool := NewGetProjectContextTool(mockManager, tempDir)

	input := &GetProjectContextInput{
		ProjectID:    "test-project-id",
		UserID:       "test-user-id",
		IncludeFiles: false,
		MaxDepth:     2,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	var output ProjectContextOutput
	err = json.Unmarshal([]byte(result), &output)
	assert.NoError(t, err)
	assert.Nil(t, output.FileStructure)

	mockManager.AssertExpectations(t)
}

func TestGetProjectContextTool_InvokableRun_InvalidInput(t *testing.T) {
	mockManager := new(MockProjectManager)
	tool := NewGetProjectContextTool(mockManager, "/test/root")

	// Test with empty project_id
	input := &GetProjectContextInput{
		ProjectID: "",
		UserID:    "test-user-id",
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	assert.NoError(t, err)
	assert.Contains(t, result, "error: project_id cannot be empty")
}

func TestGetProjectContextTool_InvokableRun_ProjectNotFound(t *testing.T) {
	mockManager := new(MockProjectManager)
	mockManager.On("GetSession", mock.Anything, "nonexistent-id", "test-user-id").Return(
		nil, assert.AnError)

	tool := NewGetProjectContextTool(mockManager, "/test/root")

	input := &GetProjectContextInput{
		ProjectID: "nonexistent-id",
		UserID:    "test-user-id",
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	assert.NoError(t, err)
	assert.Contains(t, result, "error: failed to retrieve project")

	mockManager.AssertExpectations(t)
}

func TestGetProjectContextTool_BuildFileStructure(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "file-structure-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create nested structure
	os.MkdirAll(filepath.Join(tempDir, "src", "components"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tempDir, "src", "components", "button.go"), []byte("package components"), 0644)
	os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Test"), 0644)

	// Create directories that should be skipped
	os.MkdirAll(filepath.Join(tempDir, "node_modules"), 0755)
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0755)

	mockManager := new(MockProjectManager)
	tool := &getProjectContextTool{
		projectManager: mockManager,
		projectRoot:    tempDir,
	}

	// Build file structure
	structure, err := tool.buildFileStructure(tempDir, 0, 3)

	assert.NoError(t, err)
	assert.NotNil(t, structure)
	assert.Equal(t, "directory", structure.Type)
	assert.NotEmpty(t, structure.Children)

	// Verify node_modules and .git are skipped
	for _, child := range structure.Children {
		assert.NotEqual(t, "node_modules", child.Name)
		assert.NotEqual(t, ".git", child.Name)
	}
}

func TestGetProjectContextTool_ShouldSkip(t *testing.T) {
	tool := &getProjectContextTool{}

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"hidden file", ".hidden", true},
		{"node_modules", "node_modules", true},
		{"vendor", "vendor", true},
		{"dist", "dist", true},
		{"normal file", "main.go", false},
		{"normal dir", "src", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.shouldSkip(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProjectContextTool_MaxDepthLimit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "depth-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create deep nested structure
	os.MkdirAll(filepath.Join(tempDir, "a", "b", "c", "d", "e"), 0755)

	mockManager := new(MockProjectManager)
	tool := &getProjectContextTool{
		projectManager: mockManager,
		projectRoot:    tempDir,
	}

	// Build with max depth of 2
	structure, err := tool.buildFileStructure(tempDir, 0, 2)

	assert.NoError(t, err)
	assert.NotNil(t, structure)

	// Verify depth is limited
	depth := calculateDepth(structure)
	assert.LessOrEqual(t, depth, 3) // 0-indexed, so max depth 2 means 3 levels
}

// Helper function to calculate tree depth
func calculateDepth(node *FileStructureNode) int {
	if node == nil || len(node.Children) == 0 {
		return 1
	}

	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := calculateDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return maxChildDepth + 1
}
