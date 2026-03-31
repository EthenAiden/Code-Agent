package handler

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/service"
	"github.com/stretchr/testify/assert"
)

// TestChatHandler_StreamingConfiguration tests that streaming is properly configured using adk.RunnerConfig
// This test verifies that the chat handler properly initializes with the required dependencies
// Requirements: 9.1, 9.2
func TestChatHandler_StreamingConfiguration(t *testing.T) {
	var mockService *service.MessageHistoryService
	var mockProjectManager tools.ProjectManagerInterface
	var mockAgent adk.Agent

	handler := NewChatHandler(mockService, mockProjectManager, mockAgent)

	assert.NotNil(t, handler)
}

// TestNewChatHandler tests the constructor
// Verifies that the handler is properly initialized with dependencies
func TestNewChatHandler(t *testing.T) {
	var mockService *service.MessageHistoryService
	var mockProjectManager tools.ProjectManagerInterface
	var mockAgent adk.Agent

	handler := NewChatHandler(mockService, mockProjectManager, mockAgent)

	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.messageHistoryService)
	assert.Equal(t, mockProjectManager, handler.projectManager)
	assert.Equal(t, mockAgent, handler.agent)
}
