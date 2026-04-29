package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/ethen-aiden/code-agent/model"
)

// BuildHandler handles build and preview endpoints
type BuildHandler struct {
	projectRoot string
	devServers  map[string]*DevServer
	mu          sync.RWMutex
}

// DevServer represents a running development server (backed by a Docker container)
type DevServer struct {
	ProjectID string
	Port      int
	Status    string // "starting", "running", "stopped", "error"
	StartedAt time.Time
	URL       string
}

// NewBuildHandler creates a new BuildHandler
func NewBuildHandler(projectRoot string) *BuildHandler {
	return &BuildHandler{
		projectRoot: projectRoot,
		devServers:  make(map[string]*DevServer),
	}
}

// BuildProject handles POST /api/v1/projects/:project_id/build
func (h *BuildHandler) BuildProject(ctx context.Context, c *app.RequestContext) {
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
		c.JSON(http.StatusNotFound, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "PROJECT_NOT_FOUND",
				Message:   "Project directory not found",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Check if dev server is already running
	h.mu.RLock()
	existingServer, exists := h.devServers[projectID]
	h.mu.RUnlock()

	if exists && existingServer.Status == "running" {
		c.JSON(http.StatusOK, model.APIResponse{
			Data: map[string]interface{}{
				"status":  "already_running",
				"url":     existingServer.URL,
				"message": "Development server is already running",
			},
		})
		return
	}

	// Start dev server in background
	go h.startDevServer(projectID, projectDir, userIDStr, "")

	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]interface{}{
			"status":  "starting",
			"message": "Development server is starting...",
		},
	})
}

// StartDevServer starts a development server for the project (exported for use by ChatHandler)
func (h *BuildHandler) StartDevServer(projectID, projectDir, userID, framework string) {
	h.startDevServer(projectID, projectDir, userID, framework)
}

// startDevServer starts a Docker-based development server for the project.
// It mounts the project directory into a node:20-alpine container and runs
// "npm install && npm run dev" with the Vite/Expo port exposed on the host.
func (h *BuildHandler) startDevServer(projectID, projectDir, userID, framework string) {
	port := h.findAvailablePort(5173)

	devServer := &DevServer{
		ProjectID: projectID,
		Port:      port,
		Status:    "starting",
		StartedAt: time.Now(),
		URL:       fmt.Sprintf("http://localhost:%d", port),
	}

	h.mu.Lock()
	h.devServers[projectID] = devServer
	h.mu.Unlock()

	// Check if package.json exists
	packageJSONPath := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		devServer.Status = "error"
		fmt.Printf("Error: package.json not found in %s\n", projectDir)
		return
	}

	// Resolve to absolute path — Docker requires an absolute host path for volume mounts
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		devServer.Status = "error"
		fmt.Printf("Error resolving absolute path for %s: %v\n", projectDir, err)
		return
	}

	// Convert Windows path to Docker-compatible path if needed
	mountPath := absProjectDir
	if runtime.GOOS == "windows" {
		// Convert C:\path\to\dir → /c/path/to/dir for Docker Desktop on Windows
		if len(mountPath) >= 2 && mountPath[1] == ':' {
			driveLetter := string(mountPath[0])
			rest := filepath.ToSlash(mountPath[2:])
			mountPath = "/" + strings.ToLower(driveLetter) + rest
		}
	} else {
		mountPath = filepath.ToSlash(absProjectDir)
	}

	containerName := fmt.Sprintf("code-agent-dev-%s", projectID[:8])

	// Remove any stale container with the same name
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	fmt.Printf("Starting Docker dev server for project %s on port %d...\n", projectID, port)

	// Build the dev command based on framework
	var devCmd string
	if framework == "react-native" {
		devCmd = fmt.Sprintf("npm install --prefer-offline 2>&1 && npx expo start --web --port %d 2>&1", port)
	} else {
		devCmd = fmt.Sprintf("npm install --prefer-offline 2>&1 && npm run dev -- --port %d --host 0.0.0.0 2>&1", port)
	}

	// docker run -d (detached) starts the container and returns the container ID immediately.
	// We then use "docker wait" in a goroutine to detect when it stops.
	// Note: no --rm so we can reliably call "docker wait"; we clean up manually after.
	cmd := exec.Command("docker", "run",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"-v", fmt.Sprintf("%s:/app", mountPath),
		"-w", "/app",
		"node:20-alpine",
		"sh", "-c",
		devCmd,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		devServer.Status = "error"
		fmt.Printf("Error starting Docker container for project %s: %v\n", projectID, err)
		return
	}

	// Get the container ID for later management
	out, err := exec.Command("docker", "inspect", "--format={{.Id}}", containerName).Output()
	containerID := ""
	if err == nil {
		containerID = strings.TrimSpace(string(out))
	}

	devServer.Status = "running"
	fmt.Printf("Docker dev server started for project %s at %s (container: %s)\n", projectID, devServer.URL, containerName)

	// Wait for the container to stop (blocks until exit)
	waitCmd := exec.Command("docker", "wait", containerName)
	if containerID != "" {
		waitCmd = exec.Command("docker", "wait", containerID)
	}
	_ = waitCmd.Run()

	// Clean up the stopped container
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	h.mu.Lock()
	if server, exists := h.devServers[projectID]; exists {
		server.Status = "stopped"
	}
	h.mu.Unlock()
}

// findAvailablePort finds an available port starting from the given port
func (h *BuildHandler) findAvailablePort(startPort int) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	usedPorts := make(map[int]bool)
	for _, server := range h.devServers {
		if server.Status == "running" || server.Status == "starting" {
			usedPorts[server.Port] = true
		}
	}

	port := startPort
	for {
		if !usedPorts[port] {
			return port
		}
		port++
	}
}

// GetPreviewURL handles GET /api/v1/projects/:project_id/preview
func (h *BuildHandler) GetPreviewURL(ctx context.Context, c *app.RequestContext) {
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

	// Get dev server info
	h.mu.RLock()
	devServer, exists := h.devServers[projectID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "SERVER_NOT_FOUND",
				Message:   "Development server not found. Please build the project first.",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]interface{}{
			"url":    devServer.URL,
			"status": devServer.Status,
			"port":   devServer.Port,
		},
	})
}

// StopDevServer handles POST /api/v1/projects/:project_id/stop
func (h *BuildHandler) StopDevServer(ctx context.Context, c *app.RequestContext) {
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

	// Get dev server
	h.mu.Lock()
	devServer, exists := h.devServers[projectID]
	if !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "SERVER_NOT_FOUND",
				Message:   "Development server not found",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Stop the Docker container
	if devServer.Status == "running" {
		containerName := fmt.Sprintf("code-agent-dev-%s", projectID[:8])
		if err := exec.Command("docker", "stop", containerName).Run(); err != nil {
			h.mu.Unlock()
			c.JSON(http.StatusInternalServerError, model.APIResponse{
				Error: &model.APIError{
					ErrorCode: "STOP_ERROR",
					Message:   fmt.Sprintf("Failed to stop dev server container: %v", err),
					Timestamp: time.Now().Format(time.RFC3339),
				},
			})
			return
		}
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
		devServer.Status = "stopped"
	}

	delete(h.devServers, projectID)
	h.mu.Unlock()

	c.JSON(http.StatusOK, model.APIResponse{
		Data: map[string]interface{}{
			"success": true,
			"message": "Development server stopped",
		},
	})
}
