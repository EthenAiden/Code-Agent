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
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
)

// projectDir resolves the per-project working directory.
// It reads "project_id" from the agent context and returns baseRoot/project_id/.
// Falls back to baseRoot when no project_id is present.
func projectDir(ctx context.Context, baseRoot string) string {
	if projectID, ok := agentcontext.GetTypedContextParams[string](ctx, "project_id"); ok && projectID != "" {
		dir := filepath.Join(baseRoot, projectID)
		// Ensure the project directory exists
		_ = os.MkdirAll(dir, 0755)
		return dir
	}
	return baseRoot
}

// ReadFileTool provides file reading capabilities for agents
var readFileToolInfo = &schema.ToolInfo{
	Name: "read_file",
	Desc: `This tool reads the content of a file from the project directory.
Parameters:
- path: The file path relative to the project root (required)
- start_line: The starting line number (default: 1)
- num_lines: Number of lines to read, -1 means read all lines from start_line to end (default: -1)

Returns the file content as a string. Content may be truncated if the file is very large.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"path": {
			Type:     schema.String,
			Desc:     "File path relative to project root",
			Required: true,
		},
		"start_line": {
			Type: schema.Integer,
			Desc: "Starting line number (1-indexed), defaults to 1",
		},
		"num_lines": {
			Type: schema.Integer,
			Desc: "Number of lines to read, -1 means read all remaining lines, defaults to -1",
		},
	}),
}

// ReadFileInput defines the input structure for ReadFileTool
type ReadFileInput struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	NumLines  int    `json:"num_lines"`
}

// readFileTool implements the file reading tool
type readFileTool struct {
	projectRoot string
}

// NewReadFileTool creates a new ReadFileTool instance
func NewReadFileTool(projectRoot string) tool.InvokableTool {
	return &readFileTool{
		projectRoot: projectRoot,
	}
}

func (r *readFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return readFileToolInfo, nil
}

func (r *readFileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	input := &ReadFileInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if input.Path == "" {
		return "error: path cannot be empty", nil
	}

	// Resolve per-project directory from context
	root := projectDir(ctx, r.projectRoot)

	// Validate and resolve file path
	fullPath, err := validatePathInRoot(input.Path, root)
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error()), nil
	}

	// Set defaults
	if input.StartLine <= 0 {
		input.StartLine = 1
	}
	if input.NumLines == 0 {
		input.NumLines = -1
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("error: failed to read file: %s", err.Error()), nil
	}

	// Split into lines and apply line range
	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if input.StartLine > totalLines {
		return fmt.Sprintf("error: start_line %d exceeds total lines %d", input.StartLine, totalLines), nil
	}

	startIdx := input.StartLine - 1
	endIdx := totalLines

	if input.NumLines > 0 {
		endIdx = startIdx + input.NumLines
		if endIdx > totalLines {
			endIdx = totalLines
		}
	}

	selectedLines := lines[startIdx:endIdx]
	result := strings.Join(selectedLines, "\n")

	return result, nil
}

// validatePathInRoot validates a relative file path against a specific root directory.
// It prevents directory traversal attacks and returns the absolute path.
func validatePathInRoot(path string, root string) (string, error) {
	cleanPath := filepath.Clean(path)
	fullPath := filepath.Join(root, cleanPath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("invalid project root: %w", err)
	}

	if !strings.HasPrefix(absPath, absRoot) {
		return "", fmt.Errorf("access denied: path is outside project directory")
	}

	return absPath, nil
}

// validatePath validates the file path and prevents directory traversal attacks
func (r *readFileTool) validatePath(path string) (string, error) {
	absPath, err := validatePathInRoot(path, r.projectRoot)
	if err != nil {
		return "", err
	}
	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}
	return absPath, nil
}

// WriteFileTool provides file writing capabilities for agents
var writeFileToolInfo = &schema.ToolInfo{
	Name: "write_file",
	Desc: `This tool writes content to a file in the project directory.
Parameters:
- path: The file path relative to the project root (required)
- content: The content to write to the file (required)

If the file does not exist, it will be created with permissions 0644.
If the file exists, it will be truncated and overwritten with the new content.
Only text files are supported.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"path": {
			Type:     schema.String,
			Desc:     "File path relative to project root",
			Required: true,
		},
		"content": {
			Type:     schema.String,
			Desc:     "Content to write to the file",
			Required: true,
		},
	}),
}

// WriteFileInput defines the input structure for WriteFileTool
type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// writeFileTool implements the file writing tool
type writeFileTool struct {
	projectRoot string
}

// NewWriteFileTool creates a new WriteFileTool instance
func NewWriteFileTool(projectRoot string) tool.InvokableTool {
	return &writeFileTool{
		projectRoot: projectRoot,
	}
}

func (w *writeFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return writeFileToolInfo, nil
}

func (w *writeFileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	input := &WriteFileInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if input.Path == "" {
		return "error: path cannot be empty", nil
	}

	// Resolve per-project directory from context
	root := projectDir(ctx, w.projectRoot)

	// Block sensitive files
	cleanPath := filepath.Clean(input.Path)
	sensitivePatterns := []string{".env", ".git", ".ssh", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519"}
	for _, sensitive := range sensitivePatterns {
		if strings.Contains(cleanPath, sensitive) {
			return "error: access denied: cannot write to sensitive file", nil
		}
	}

	// Validate and resolve file path
	fullPath, err := validatePathInRoot(input.Path, root)
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error()), nil
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("error: failed to create directory: %s", err.Error()), nil
	}

	// Write file content
	if err := os.WriteFile(fullPath, []byte(input.Content), 0644); err != nil {
		return fmt.Sprintf("error: failed to write file: %s", err.Error()), nil
	}

	return fmt.Sprintf("success: file written to %s", input.Path), nil
}

// validatePath delegates to the shared validatePathInRoot helper.
func (w *writeFileTool) validatePath(path string) (string, error) {
	return validatePathInRoot(path, w.projectRoot)
}

// ListDirectoryTool provides directory listing capabilities for agents
var listDirectoryToolInfo = &schema.ToolInfo{
	Name: "list_directory",
	Desc: `This tool lists the contents of a directory in the project.
Parameters:
- path: The directory path relative to the project root (required)
- recursive: Whether to list subdirectories recursively (default: false)

Returns a list of files and directories with their types (file/directory).`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"path": {
			Type:     schema.String,
			Desc:     "Directory path relative to project root",
			Required: true,
		},
		"recursive": {
			Type: schema.Boolean,
			Desc: "Whether to list subdirectories recursively, defaults to false",
		},
	}),
}

// ListDirectoryInput defines the input structure for ListDirectoryTool
type ListDirectoryInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

// listDirectoryTool implements the directory listing tool
type listDirectoryTool struct {
	projectRoot string
}

// NewListDirectoryTool creates a new ListDirectoryTool instance
func NewListDirectoryTool(projectRoot string) tool.InvokableTool {
	return &listDirectoryTool{
		projectRoot: projectRoot,
	}
}

func (l *listDirectoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return listDirectoryToolInfo, nil
}

func (l *listDirectoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	input := &ListDirectoryInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if input.Path == "" {
		input.Path = "."
	}

	// Resolve per-project directory from context
	root := projectDir(ctx, l.projectRoot)

	// Validate and resolve directory path
	fullPath, err := validatePathInRoot(input.Path, root)
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error()), nil
	}

	// Verify it's a directory
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return fmt.Sprintf("error: directory does not exist: %s", input.Path), nil
	}
	if !info.IsDir() {
		return fmt.Sprintf("error: path is not a directory: %s", input.Path), nil
	}

	// List directory contents
	var result strings.Builder
	if input.Recursive {
		err = l.listRecursive(fullPath, "", &result)
	} else {
		err = l.listFlat(fullPath, &result)
	}

	if err != nil {
		return fmt.Sprintf("error: %s", err.Error()), nil
	}

	return result.String(), nil
}

// listFlat lists directory contents non-recursively
func (l *listDirectoryTool) listFlat(dirPath string, result *strings.Builder) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		entryType := "file"
		if entry.IsDir() {
			entryType = "directory"
		}
		result.WriteString(fmt.Sprintf("%s (%s)\n", entry.Name(), entryType))
	}

	return nil
}

// listRecursive lists directory contents recursively
func (l *listDirectoryTool) listRecursive(dirPath string, prefix string, result *strings.Builder) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for i, entry := range entries {
		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		entryType := "file"
		if entry.IsDir() {
			entryType = "directory"
		}

		result.WriteString(fmt.Sprintf("%s%s%s (%s)\n", prefix, connector, entry.Name(), entryType))

		if entry.IsDir() {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			subPath := filepath.Join(dirPath, entry.Name())
			if err := l.listRecursive(subPath, newPrefix, result); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePath delegates to the shared validatePathInRoot helper.
func (l *listDirectoryTool) validatePath(path string) (string, error) {
	return validatePathInRoot(path, l.projectRoot)
}
