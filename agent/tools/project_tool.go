package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/ethen-aiden/code-agent/model"
)

// GetProjectContextTool provides project context retrieval capabilities for agents
var getProjectContextToolInfo = &schema.ToolInfo{
	Name: "get_project_context",
	Desc: `This tool retrieves project context including metadata, file structure, and existing code.
Parameters:
- project_id: The project ID (required)
- user_id: The user ID (required)
- include_files: Whether to include file structure (default: true)
- max_depth: Maximum directory depth for file structure (default: 3, max: 10)

Returns project metadata, file structure, and summary information.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"project_id": {
			Type:     schema.String,
			Desc:     "Project ID to retrieve context for",
			Required: true,
		},
		"user_id": {
			Type:     schema.String,
			Desc:     "User ID who owns the project",
			Required: true,
		},
		"include_files": {
			Type: schema.Boolean,
			Desc: "Whether to include file structure, defaults to true",
		},
		"max_depth": {
			Type: schema.Integer,
			Desc: "Maximum directory depth for file structure (default: 3, max: 10)",
		},
	}),
}

// GetProjectContextInput defines the input structure for GetProjectContextTool
type GetProjectContextInput struct {
	ProjectID    string `json:"project_id"`
	UserID       string `json:"user_id"`
	IncludeFiles bool   `json:"include_files"`
	MaxDepth     int    `json:"max_depth"`
}

// ProjectContextOutput defines the output structure for GetProjectContextTool
type ProjectContextOutput struct {
	ProjectID     string             `json:"project_id"`
	UserID        string             `json:"user_id"`
	Framework     string             `json:"framework"` // "vue3", "react", "react-native", or ""
	MessageCount  int                `json:"message_count"`
	CreatedAt     string             `json:"created_at"`
	FileStructure *FileStructureNode `json:"file_structure,omitempty"`
	Summary       string             `json:"summary"`
}

// FileStructureNode represents a node in the file structure tree
type FileStructureNode struct {
	Name     string               `json:"name"`
	Type     string               `json:"type"` // "file" or "directory"
	Path     string               `json:"path"`
	Children []*FileStructureNode `json:"children,omitempty"`
}

// ProjectManagerInterface defines the interface for project management operations
type ProjectManagerInterface interface {
	GetSession(ctx context.Context, conversationID string, userID string) (*model.SessionDetails, error)
	CreateSession(ctx context.Context, userID string) (string, error)
	ListSessions(ctx context.Context, userID string, limit, offset int) ([]model.SessionSummary, error)
	DeleteSession(ctx context.Context, conversationID string, userID string) error
	SetFramework(ctx context.Context, conversationID string, userID string, framework string) error
}

// getProjectContextTool implements the project context retrieval tool
type getProjectContextTool struct {
	projectManager ProjectManagerInterface
	projectRoot    string
}

// NewGetProjectContextTool creates a new GetProjectContextTool instance
func NewGetProjectContextTool(projectManager ProjectManagerInterface, projectRoot string) tool.InvokableTool {
	return &getProjectContextTool{
		projectManager: projectManager,
		projectRoot:    projectRoot,
	}
}

func (p *getProjectContextTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return getProjectContextToolInfo, nil
}

func (p *getProjectContextTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	input := &GetProjectContextInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate inputs
	if input.ProjectID == "" {
		return "error: project_id cannot be empty", nil
	}
	if input.UserID == "" {
		return "error: user_id cannot be empty", nil
	}

	// Set defaults
	if input.MaxDepth <= 0 {
		input.MaxDepth = 3
	}
	if input.MaxDepth > 10 {
		input.MaxDepth = 10
	}

	// Retrieve project details from project manager
	details, err := p.projectManager.GetSession(ctx, input.ProjectID, input.UserID)
	if err != nil {
		return fmt.Sprintf("error: failed to retrieve project: %s", err.Error()), nil
	}

	// Build output structure
	output := &ProjectContextOutput{
		ProjectID:    input.ProjectID,
		UserID:       input.UserID,
		Framework:    details.Framework,
		MessageCount: details.MessageCount,
		CreatedAt:    details.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	// Include file structure if requested
	if input.IncludeFiles {
		fileStructure, err := p.buildFileStructure(p.projectRoot, 0, input.MaxDepth)
		if err != nil {
			return fmt.Sprintf("error: failed to build file structure: %s", err.Error()), nil
		}
		output.FileStructure = fileStructure
	}

	// Generate summary
	frameworkNote := ""
	if details.Framework != "" {
		frameworkNote = fmt.Sprintf(" Framework: %s.", details.Framework)
	} else {
		frameworkNote = " Framework: not yet selected."
	}
	output.Summary = fmt.Sprintf("Project %s has %d messages. Created at %s.%s",
		input.ProjectID, details.MessageCount, output.CreatedAt, frameworkNote)

	// Serialize output to JSON
	result, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize output: %w", err)
	}

	return string(result), nil
}

// buildFileStructure recursively builds the file structure tree
func (p *getProjectContextTool) buildFileStructure(dirPath string, currentDepth int, maxDepth int) (*FileStructureNode, error) {
	// Check depth limit
	if currentDepth > maxDepth {
		return nil, nil
	}

	// Get directory info
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// Get relative path from project root
	relPath, err := filepath.Rel(p.projectRoot, dirPath)
	if err != nil {
		relPath = filepath.Base(dirPath)
	}
	if relPath == "." {
		relPath = ""
	}

	// Create node
	node := &FileStructureNode{
		Name: filepath.Base(dirPath),
		Path: relPath,
	}

	// If it's a file, return immediately
	if !info.IsDir() {
		node.Type = "file"
		return node, nil
	}

	// It's a directory
	node.Type = "directory"

	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Process children
	for _, entry := range entries {
		// Skip hidden files and common ignore patterns
		if p.shouldSkip(entry.Name()) {
			continue
		}

		childPath := filepath.Join(dirPath, entry.Name())
		childNode, err := p.buildFileStructure(childPath, currentDepth+1, maxDepth)
		if err != nil {
			// Log error but continue with other entries
			continue
		}
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}

	return node, nil
}

// shouldSkip determines if a file or directory should be skipped
func (p *getProjectContextTool) shouldSkip(name string) bool {
	// Skip hidden files
	if strings.HasPrefix(name, ".") {
		return true
	}

	// Skip common directories to ignore
	skipDirs := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		"target":       true,
		"__pycache__":  true,
		".git":         true,
		".vscode":      true,
		".idea":        true,
	}

	return skipDirs[name]
}
