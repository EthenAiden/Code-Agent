package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecuteCodeTool_Python(t *testing.T) {
	// Create temp work directory
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	tests := []struct {
		name        string
		code        string
		wantContain string
		wantError   bool
	}{
		{
			name:        "simple print",
			code:        `print("Hello, World!")`,
			wantContain: "Hello, World!",
			wantError:   false,
		},
		{
			name: "arithmetic operations",
			code: `
result = 2 + 2
print(f"Result: {result}")
`,
			wantContain: "Result: 4",
			wantError:   false,
		},
		{
			name:        "syntax error",
			code:        `print("missing closing quote)`,
			wantContain: "error",
			wantError:   false, // Tool returns error as string, not Go error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ExecuteCodeInput{
				Language: "python",
				Code:     tt.code,
				Timeout:  5,
			}
			inputJSON, _ := json.Marshal(input)

			result, err := tool.InvokableRun(context.Background(), string(inputJSON))
			if err != nil {
				t.Fatalf("InvokableRun() error = %v", err)
			}

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("InvokableRun() result = %v, want to contain %v", result, tt.wantContain)
			}
		})
	}
}

func TestExecuteCodeTool_JavaScript(t *testing.T) {
	// Create temp work directory
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	tests := []struct {
		name        string
		code        string
		wantContain string
		wantError   bool
	}{
		{
			name:        "simple console.log",
			code:        `console.log("Hello, JavaScript!");`,
			wantContain: "Hello, JavaScript!",
			wantError:   false,
		},
		{
			name: "arithmetic operations",
			code: `
const result = 10 * 5;
console.log("Result:", result);
`,
			wantContain: "Result: 50",
			wantError:   false,
		},
		{
			name:        "syntax error",
			code:        `console.log("missing closing paren"`,
			wantContain: "error",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ExecuteCodeInput{
				Language: "javascript",
				Code:     tt.code,
				Timeout:  5,
			}
			inputJSON, _ := json.Marshal(input)

			result, err := tool.InvokableRun(context.Background(), string(inputJSON))
			if err != nil {
				t.Fatalf("InvokableRun() error = %v", err)
			}

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("InvokableRun() result = %v, want to contain %v", result, tt.wantContain)
			}
		})
	}
}

func TestExecuteCodeTool_Go(t *testing.T) {
	// Create temp work directory
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	tests := []struct {
		name        string
		code        string
		wantContain string
		wantError   bool
	}{
		{
			name: "simple main",
			code: `package main

import "fmt"

func main() {
	fmt.Println("Hello, Go!")
}`,
			wantContain: "Hello, Go!",
			wantError:   false,
		},
		{
			name: "arithmetic operations",
			code: `package main

import "fmt"

func main() {
	result := 7 * 6
	fmt.Printf("Result: %d\n", result)
}`,
			wantContain: "Result: 42",
			wantError:   false,
		},
		{
			name: "syntax error",
			code: `package main

func main() {
	fmt.Println("missing import")
}`,
			wantContain: "error",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ExecuteCodeInput{
				Language: "go",
				Code:     tt.code,
				Timeout:  10, // Go compilation may take longer
			}
			inputJSON, _ := json.Marshal(input)

			result, err := tool.InvokableRun(context.Background(), string(inputJSON))
			if err != nil {
				t.Fatalf("InvokableRun() error = %v", err)
			}

			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("InvokableRun() result = %v, want to contain %v", result, tt.wantContain)
			}
		})
	}
}

func TestExecuteCodeTool_Timeout(t *testing.T) {
	// Create temp work directory
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	// Python code that sleeps for 5 seconds
	input := ExecuteCodeInput{
		Language: "python",
		Code:     `import time; time.sleep(5); print("Done")`,
		Timeout:  1, // 1 second timeout
	}
	inputJSON, _ := json.Marshal(input)

	start := time.Now()
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	// Should timeout and return error message
	if !strings.Contains(result, "timeout") {
		t.Errorf("Expected timeout error, got: %v", result)
	}

	// Should complete within reasonable time (not wait full 5 seconds)
	if elapsed > 3*time.Second {
		t.Errorf("Timeout took too long: %v", elapsed)
	}
}

func TestExecuteCodeTool_InvalidLanguage(t *testing.T) {
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	input := ExecuteCodeInput{
		Language: "ruby",
		Code:     `puts "Hello"`,
		Timeout:  5,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	if !strings.Contains(result, "unsupported language") {
		t.Errorf("Expected unsupported language error, got: %v", result)
	}
}

func TestExecuteCodeTool_EmptyCode(t *testing.T) {
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	input := ExecuteCodeInput{
		Language: "python",
		Code:     "",
		Timeout:  5,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	if !strings.Contains(result, "code cannot be empty") {
		t.Errorf("Expected empty code error, got: %v", result)
	}
}

func TestExecuteCodeTool_DefaultTimeout(t *testing.T) {
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	// Test with timeout = 0 (should use default)
	input := ExecuteCodeInput{
		Language: "python",
		Code:     `print("Testing default timeout")`,
		Timeout:  0,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	if !strings.Contains(result, "Testing default timeout") {
		t.Errorf("Expected successful execution with default timeout, got: %v", result)
	}
}

func TestExecuteCodeTool_MaxTimeout(t *testing.T) {
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	// Test with timeout > 300 (should cap at 300)
	input := ExecuteCodeInput{
		Language: "python",
		Code:     `print("Testing max timeout")`,
		Timeout:  500, // Should be capped at 300
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	if !strings.Contains(result, "Testing max timeout") {
		t.Errorf("Expected successful execution with capped timeout, got: %v", result)
	}
}

func TestExecuteCodeTool_Info(t *testing.T) {
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}

	if info.Name != "execute_code" {
		t.Errorf("Info().Name = %v, want execute_code", info.Name)
	}

	if info.Desc == "" {
		t.Error("Info().Desc should not be empty")
	}

	if info.ParamsOneOf == nil {
		t.Error("Info().ParamsOneOf should not be nil")
	}
}

func TestExecuteCodeTool_WorkDirIsolation(t *testing.T) {
	// Create temp work directory
	workDir := t.TempDir()
	tool := NewExecuteCodeTool(workDir)

	// Create a test file in work directory
	testFile := "test-file.txt"
	testFilePath := workDir + "/" + testFile
	if err := os.WriteFile(testFilePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to access the file from Python code (should not be accessible)
	input := ExecuteCodeInput{
		Language: "python",
		Code: `
import os
print("Files:", os.listdir("."))
`,
		Timeout: 5,
	}
	inputJSON, _ := json.Marshal(input)

	result, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	// The code should run in a temp directory, not the work directory
	// So it should not see the test file
	if strings.Contains(result, testFile) {
		t.Errorf("Code should not have access to work directory files, got: %v", result)
	}
}
