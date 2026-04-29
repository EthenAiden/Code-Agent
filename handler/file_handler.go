package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/ethen-aiden/code-agent/model"
)

// FileHandler handles file-related endpoints
type FileHandler struct {
	projectRoot string
}

// NewFileHandler creates a new FileHandler
func NewFileHandler(projectRoot string) *FileHandler {
	return &FileHandler{
		projectRoot: projectRoot,
	}
}

// FileNode represents a file or directory in the file tree
type FileNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Type     string     `json:"type"` // "file" or "directory"
	Size     int64      `json:"size,omitempty"`
	Modified string     `json:"modified,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

// GetFileTree handles GET /api/v1/projects/:project_id/files
func (h *FileHandler) GetFileTree(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context
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

	// Build project directory path
	projectDir := filepath.Join(h.projectRoot, projectID)

	// Check if project directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, model.APIResponse{
			Data: map[string]interface{}{
				"files": []FileNode{},
			},
		})
		return
	}

	// Build file tree
	files, err := h.buildFileTree(projectDir, projectDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "FILE_TREE_ERROR",
				Message:   fmt.Sprintf("Failed to build file tree: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	_ = userIDStr // Use userIDStr to avoid unused variable error

	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]interface{}{
			"files": files,
		},
	})
}

// buildFileTree recursively builds the file tree
func (h *FileHandler) buildFileTree(rootPath, currentPath string) ([]FileNode, error) {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	var nodes []FileNode

	for _, entry := range entries {
		// Skip hidden files and common ignore patterns
		if strings.HasPrefix(entry.Name(), ".") ||
			entry.Name() == "node_modules" ||
			entry.Name() == "dist" ||
			entry.Name() == "build" {
			continue
		}

		fullPath := filepath.Join(currentPath, entry.Name())
		relativePath, err := filepath.Rel(rootPath, fullPath)
		if err != nil {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		node := FileNode{
			Name:     entry.Name(),
			Path:     filepath.ToSlash(relativePath),
			Modified: info.ModTime().Format(time.RFC3339),
		}

		if entry.IsDir() {
			node.Type = "directory"
			// Recursively get children
			children, err := h.buildFileTree(rootPath, fullPath)
			if err == nil {
				node.Children = children
			}
		} else {
			node.Type = "file"
			node.Size = info.Size()
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetFileContent handles GET /api/v1/projects/:project_id/files/content
func (h *FileHandler) GetFileContent(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context
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

	// Get file path from query parameter
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "path query parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Build full file path
	projectDir := filepath.Join(h.projectRoot, projectID)
	fullPath := filepath.Join(projectDir, filepath.FromSlash(filePath))

	// Validate path is within project directory
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "PATH_ERROR",
				Message:   "Failed to resolve project directory",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	absFilePath, err := filepath.Abs(fullPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_PATH",
				Message:   "Invalid file path",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	if !strings.HasPrefix(absFilePath, absProjectDir) {
		c.JSON(http.StatusForbidden, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "ACCESS_DENIED",
				Message:   "Access denied: path is outside project directory",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Read file content
	content, err := os.ReadFile(absFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, model.APIResponse{
				Error: &model.APIError{
					ErrorCode: "FILE_NOT_FOUND",
					Message:   "File not found",
					Timestamp: time.Now().Format(time.RFC3339),
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "FILE_READ_ERROR",
				Message:   fmt.Sprintf("Failed to read file: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	_ = userIDStr // Use userIDStr to avoid unused variable error

	// Determine language from file extension
	ext := filepath.Ext(filePath)
	language := strings.TrimPrefix(ext, ".")

	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]interface{}{
			"path":     filePath,
			"content":  string(content),
			"language": language,
		},
	})
}

// UpdateFileContentRequest represents the request body for updating file content
type UpdateFileContentRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// UpdateFileContent handles PUT /api/v1/projects/:project_id/files/content
func (h *FileHandler) UpdateFileContent(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context
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

	// Parse request body
	var req UpdateFileContentRequest
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

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "path is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Build full file path
	projectDir := filepath.Join(h.projectRoot, projectID)
	fullPath := filepath.Join(projectDir, filepath.FromSlash(req.Path))

	// Validate path is within project directory
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "PATH_ERROR",
				Message:   "Failed to resolve project directory",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	absFilePath, err := filepath.Abs(fullPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_PATH",
				Message:   "Invalid file path",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	if !strings.HasPrefix(absFilePath, absProjectDir) {
		c.JSON(http.StatusForbidden, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "ACCESS_DENIED",
				Message:   "Access denied: path is outside project directory",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(absFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "DIRECTORY_CREATE_ERROR",
				Message:   fmt.Sprintf("Failed to create directory: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Write file content
	if err := os.WriteFile(absFilePath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "FILE_WRITE_ERROR",
				Message:   fmt.Sprintf("Failed to write file: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	_ = userIDStr // Use userIDStr to avoid unused variable error

	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]interface{}{
			"success": true,
			"path":    req.Path,
		},
	})
}

// WatchFiles handles WebSocket connection for real-time file updates
func (h *FileHandler) WatchFiles(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context
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

	_ = userIDStr // Use userIDStr to avoid unused variable error

	// TODO: Implement WebSocket connection for file watching
	// This will be implemented in the next phase
	c.JSON(http.StatusNotImplemented, model.APIResponse{
		Error: &model.APIError{
			ErrorCode: "NOT_IMPLEMENTED",
			Message:   "WebSocket file watching not yet implemented",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})
}

// FileWatcher manages file system watching for a project
type FileWatcher struct {
	projectID   string
	projectRoot string
	lastScan    map[string]fs.FileInfo
}

// NewFileWatcher creates a new FileWatcher
func NewFileWatcher(projectID, projectRoot string) *FileWatcher {
	return &FileWatcher{
		projectID:   projectID,
		projectRoot: filepath.Join(projectRoot, projectID),
		lastScan:    make(map[string]fs.FileInfo),
	}
}

// FileChangeEvent represents a file change event
type FileChangeEvent struct {
	Type    string `json:"type"` // "file_created", "file_updated", "file_deleted"
	Path    string `json:"path"` // Relative path from project root
	Content string `json:"content,omitempty"`
}

// ScanForChanges scans the project directory for changes
func (fw *FileWatcher) ScanForChanges() ([]FileChangeEvent, error) {
	var events []FileChangeEvent
	currentScan := make(map[string]fs.FileInfo)

	// Walk the project directory
	err := filepath.Walk(fw.projectRoot, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(fw.projectRoot, path)
		if err != nil {
			return err
		}

		currentScan[relPath] = info

		// Check if file is new or modified
		if lastInfo, exists := fw.lastScan[relPath]; exists {
			if lastInfo.ModTime().Before(info.ModTime()) {
				// File was modified
				content, _ := os.ReadFile(path)
				events = append(events, FileChangeEvent{
					Type:    "file_updated",
					Path:    filepath.ToSlash(relPath),
					Content: string(content),
				})
			}
		} else {
			// File is new
			content, _ := os.ReadFile(path)
			events = append(events, FileChangeEvent{
				Type:    "file_created",
				Path:    filepath.ToSlash(relPath),
				Content: string(content),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check for deleted files
	for relPath := range fw.lastScan {
		if _, exists := currentScan[relPath]; !exists {
			events = append(events, FileChangeEvent{
				Type: "file_deleted",
				Path: filepath.ToSlash(relPath),
			})
		}
	}

	fw.lastScan = currentScan
	return events, nil
}

// BroadcastFileChange broadcasts a file change event to all connected clients
func BroadcastFileChange(projectID string, event FileChangeEvent) error {
	// TODO: Implement WebSocket broadcasting
	// This will be implemented when WebSocket support is added
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// For now, just log the event
	fmt.Printf("File change event for project %s: %s\n", projectID, string(eventJSON))
	return nil
}
