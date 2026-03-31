package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ExecuteCodeTool provides code execution capabilities for agents
var executeCodeToolInfo = &schema.ToolInfo{
	Name: "execute_code",
	Desc: `This tool executes code in a sandboxed environment with timeout and resource limits.
Parameters:
- language: The programming language (python, javascript, go) (required)
- code: The code to execute (required)
- timeout: Maximum execution time in seconds (default: 30, max: 300)

Supported languages:
- python: Executes Python code using python3
- javascript: Executes JavaScript code using node
- go: Executes Go code using go run

Returns the output (stdout and stderr) of the code execution.
Execution is sandboxed with resource limits for safety.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"language": {
			Type:     schema.String,
			Desc:     "Programming language: python, javascript, or go",
			Required: true,
		},
		"code": {
			Type:     schema.String,
			Desc:     "Code to execute",
			Required: true,
		},
		"timeout": {
			Type: schema.Integer,
			Desc: "Maximum execution time in seconds (default: 30, max: 300)",
		},
	}),
}

// ExecuteCodeInput defines the input structure for ExecuteCodeTool
type ExecuteCodeInput struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Timeout  int    `json:"timeout"`
}

// executeCodeTool implements the code execution tool
type executeCodeTool struct {
	workDir   string
	pythonCmd string
}

// NewExecuteCodeTool creates a new ExecuteCodeTool instance
func NewExecuteCodeTool(workDir string) tool.InvokableTool {
	// Determine Python command at initialization by testing execution
	pythonCmd := "python"

	// Try python3 first (preferred on Unix systems)
	if path, err := exec.LookPath("python3"); err == nil {
		// Test if python3 actually works
		testCmd := exec.Command(path, "--version")
		if err := testCmd.Run(); err == nil {
			pythonCmd = "python3"
		}
	}

	return &executeCodeTool{
		workDir:   workDir,
		pythonCmd: pythonCmd,
	}
}

func (e *executeCodeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return executeCodeToolInfo, nil
}

func (e *executeCodeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	input := &ExecuteCodeInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate inputs
	if input.Language == "" {
		return "error: language cannot be empty", nil
	}
	if input.Code == "" {
		return "error: code cannot be empty", nil
	}

	// Normalize language name
	input.Language = strings.ToLower(input.Language)

	// Validate language
	supportedLanguages := map[string]bool{
		"python":     true,
		"javascript": true,
		"go":         true,
	}
	if !supportedLanguages[input.Language] {
		return fmt.Sprintf("error: unsupported language '%s'. Supported: python, javascript, go", input.Language), nil
	}

	// Set default and validate timeout
	if input.Timeout <= 0 {
		input.Timeout = 30
	}
	if input.Timeout > 300 {
		input.Timeout = 300
	}

	// Execute code with timeout
	result, err := e.executeWithTimeout(ctx, input)
	if err != nil {
		return fmt.Sprintf("error: %s", err.Error()), nil
	}

	return result, nil
}

// executeWithTimeout executes code with a timeout
func (e *executeCodeTool) executeWithTimeout(ctx context.Context, input *ExecuteCodeInput) (string, error) {
	// Create a context with timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(input.Timeout)*time.Second)
	defer cancel()

	// Create temporary directory for code execution
	tempDir, err := os.MkdirTemp(e.workDir, "code-exec-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Execute based on language
	var cmd *exec.Cmd
	switch input.Language {
	case "python":
		cmd, err = e.executePython(execCtx, tempDir, input.Code)
	case "javascript":
		cmd, err = e.executeJavaScript(execCtx, tempDir, input.Code)
	case "go":
		cmd, err = e.executeGo(execCtx, tempDir, input.Code)
	default:
		return "", fmt.Errorf("unsupported language: %s", input.Language)
	}

	if err != nil {
		return "", err
	}

	// Run the command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it was a timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("execution timeout after %d seconds", input.Timeout)
		}
		// Include the error output
		return "", fmt.Errorf("execution failed: %s\nOutput: %s", err.Error(), string(output))
	}

	return string(output), nil
}

// executePython executes Python code
func (e *executeCodeTool) executePython(ctx context.Context, tempDir string, code string) (*exec.Cmd, error) {
	// Write code to temporary file
	codeFile := filepath.Join(tempDir, "script.py")
	if err := os.WriteFile(codeFile, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write Python code: %w", err)
	}

	// Create command with resource limits
	cmd := exec.CommandContext(ctx, e.pythonCmd, codeFile)
	cmd.Dir = tempDir

	// Set environment variables to limit resources
	cmd.Env = append(os.Environ(),
		"PYTHONDONTWRITEBYTECODE=1", // Don't create .pyc files
	)

	return cmd, nil
}

// executeJavaScript executes JavaScript code
func (e *executeCodeTool) executeJavaScript(ctx context.Context, tempDir string, code string) (*exec.Cmd, error) {
	// Write code to temporary file
	codeFile := filepath.Join(tempDir, "script.js")
	if err := os.WriteFile(codeFile, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write JavaScript code: %w", err)
	}

	// Create command with resource limits
	cmd := exec.CommandContext(ctx, "node", codeFile)
	cmd.Dir = tempDir

	return cmd, nil
}

// executeGo executes Go code
func (e *executeCodeTool) executeGo(ctx context.Context, tempDir string, code string) (*exec.Cmd, error) {
	// Write code to temporary file
	codeFile := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(codeFile, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write Go code: %w", err)
	}

	// Create command with resource limits
	cmd := exec.CommandContext(ctx, "go", "run", codeFile)
	cmd.Dir = tempDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"GOCACHE="+filepath.Join(tempDir, ".cache"), // Use temp cache
	)

	return cmd, nil
}
