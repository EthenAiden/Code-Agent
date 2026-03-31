package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDir creates a temporary test directory with sample files
func setupTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "file_tools_test_*")
	require.NoError(t, err)

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line 1\nline 2\nline 3\nline 4\nline 5"
	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create subdirectory with files
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	subFile := filepath.Join(subDir, "subfile.txt")
	err = os.WriteFile(subFile, []byte("subfile content"), 0644)
	require.NoError(t, err)

	return tmpDir
}

func TestReadFileTool_Success(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewReadFileTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name           string
		input          ReadFileInput
		expectedOutput string
	}{
		{
			name: "read entire file",
			input: ReadFileInput{
				Path:      "test.txt",
				StartLine: 1,
				NumLines:  -1,
			},
			expectedOutput: "line 1\nline 2\nline 3\nline 4\nline 5",
		},
		{
			name: "read first 3 lines",
			input: ReadFileInput{
				Path:      "test.txt",
				StartLine: 1,
				NumLines:  3,
			},
			expectedOutput: "line 1\nline 2\nline 3",
		},
		{
			name: "read from line 2 to end",
			input: ReadFileInput{
				Path:      "test.txt",
				StartLine: 2,
				NumLines:  -1,
			},
			expectedOutput: "line 2\nline 3\nline 4\nline 5",
		},
		{
			name: "read 2 lines starting from line 3",
			input: ReadFileInput{
				Path:      "test.txt",
				StartLine: 3,
				NumLines:  2,
			},
			expectedOutput: "line 3\nline 4",
		},
		{
			name: "read file in subdirectory",
			input: ReadFileInput{
				Path:      "subdir/subfile.txt",
				StartLine: 1,
				NumLines:  -1,
			},
			expectedOutput: "subfile content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			output, err := tool.InvokableRun(ctx, string(inputJSON))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, output)
		})
	}
}

func TestReadFileTool_Errors(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewReadFileTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name          string
		input         ReadFileInput
		expectedError string
	}{
		{
			name: "empty path",
			input: ReadFileInput{
				Path: "",
			},
			expectedError: "error: path cannot be empty",
		},
		{
			name: "file does not exist",
			input: ReadFileInput{
				Path: "nonexistent.txt",
			},
			expectedError: "error: file does not exist",
		},
		{
			name: "directory traversal attempt",
			input: ReadFileInput{
				Path: "../../../etc/passwd",
			},
			expectedError: "error: access denied: path is outside project directory",
		},
		{
			name: "start line exceeds total lines",
			input: ReadFileInput{
				Path:      "test.txt",
				StartLine: 100,
			},
			expectedError: "error: start_line 100 exceeds total lines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			output, err := tool.InvokableRun(ctx, string(inputJSON))
			require.NoError(t, err)
			assert.Contains(t, output, tt.expectedError)
		})
	}
}

func TestWriteFileTool_Success(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewWriteFileTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name   string
		input  WriteFileInput
		verify func(t *testing.T)
	}{
		{
			name: "write new file",
			input: WriteFileInput{
				Path:    "newfile.txt",
				Content: "new content",
			},
			verify: func(t *testing.T) {
				content, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
				require.NoError(t, err)
				assert.Equal(t, "new content", string(content))
			},
		},
		{
			name: "overwrite existing file",
			input: WriteFileInput{
				Path:    "test.txt",
				Content: "overwritten content",
			},
			verify: func(t *testing.T) {
				content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
				require.NoError(t, err)
				assert.Equal(t, "overwritten content", string(content))
			},
		},
		{
			name: "write file in new subdirectory",
			input: WriteFileInput{
				Path:    "newdir/newfile.txt",
				Content: "nested content",
			},
			verify: func(t *testing.T) {
				content, err := os.ReadFile(filepath.Join(tmpDir, "newdir", "newfile.txt"))
				require.NoError(t, err)
				assert.Equal(t, "nested content", string(content))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			output, err := tool.InvokableRun(ctx, string(inputJSON))
			require.NoError(t, err)
			assert.Contains(t, output, "success")

			tt.verify(t)
		})
	}
}

func TestWriteFileTool_Errors(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewWriteFileTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name          string
		input         WriteFileInput
		expectedError string
	}{
		{
			name: "empty path",
			input: WriteFileInput{
				Path:    "",
				Content: "content",
			},
			expectedError: "error: path cannot be empty",
		},
		{
			name: "directory traversal attempt",
			input: WriteFileInput{
				Path:    "../../../tmp/malicious.txt",
				Content: "malicious content",
			},
			expectedError: "error: access denied: path is outside project directory",
		},
		{
			name: "write to .env file",
			input: WriteFileInput{
				Path:    ".env",
				Content: "SECRET=value",
			},
			expectedError: "error: access denied: cannot write to sensitive file",
		},
		{
			name: "write to .git directory",
			input: WriteFileInput{
				Path:    ".git/config",
				Content: "malicious config",
			},
			expectedError: "error: access denied: cannot write to sensitive file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			output, err := tool.InvokableRun(ctx, string(inputJSON))
			require.NoError(t, err)
			assert.Contains(t, output, tt.expectedError)
		})
	}
}

func TestListDirectoryTool_Flat(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	input := ListDirectoryInput{
		Path:      ".",
		Recursive: false,
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := tool.InvokableRun(ctx, string(inputJSON))
	require.NoError(t, err)

	// Verify output contains expected entries
	assert.Contains(t, output, "test.txt (file)")
	assert.Contains(t, output, "subdir (directory)")
	// Should not contain nested files in flat mode
	assert.NotContains(t, output, "subfile.txt")
}

func TestListDirectoryTool_Recursive(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	input := ListDirectoryInput{
		Path:      ".",
		Recursive: true,
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := tool.InvokableRun(ctx, string(inputJSON))
	require.NoError(t, err)

	// Verify output contains expected entries
	assert.Contains(t, output, "test.txt (file)")
	assert.Contains(t, output, "subdir (directory)")
	// Should contain nested files in recursive mode
	assert.Contains(t, output, "subfile.txt (file)")
}

func TestListDirectoryTool_Subdirectory(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	input := ListDirectoryInput{
		Path:      "subdir",
		Recursive: false,
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := tool.InvokableRun(ctx, string(inputJSON))
	require.NoError(t, err)

	// Verify output contains only subdirectory contents
	assert.Contains(t, output, "subfile.txt (file)")
	assert.NotContains(t, output, "test.txt")
}

func TestListDirectoryTool_Errors(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name          string
		input         ListDirectoryInput
		expectedError string
	}{
		{
			name: "directory does not exist",
			input: ListDirectoryInput{
				Path: "nonexistent",
			},
			expectedError: "error: directory does not exist",
		},
		{
			name: "directory traversal attempt",
			input: ListDirectoryInput{
				Path: "../../../etc",
			},
			expectedError: "error: access denied: path is outside project directory",
		},
		{
			name: "path is a file not directory",
			input: ListDirectoryInput{
				Path: "test.txt",
			},
			expectedError: "error: path is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			output, err := tool.InvokableRun(ctx, string(inputJSON))
			require.NoError(t, err)
			assert.Contains(t, output, tt.expectedError)
		})
	}
}

func TestReadFileTool_Info(t *testing.T) {
	tool := NewReadFileTool("/tmp")
	ctx := context.Background()

	info, err := tool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "read_file", info.Name)
	assert.NotEmpty(t, info.Desc)
}

func TestWriteFileTool_Info(t *testing.T) {
	tool := NewWriteFileTool("/tmp")
	ctx := context.Background()

	info, err := tool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "write_file", info.Name)
	assert.NotEmpty(t, info.Desc)
}

func TestListDirectoryTool_Info(t *testing.T) {
	tool := NewListDirectoryTool("/tmp")
	ctx := context.Background()

	info, err := tool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "list_directory", info.Name)
	assert.NotEmpty(t, info.Desc)
}

func TestReadFileTool_DefaultValues(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewReadFileTool(tmpDir)
	ctx := context.Background()

	// Test with zero values - should use defaults
	input := ReadFileInput{
		Path:      "test.txt",
		StartLine: 0, // Should default to 1
		NumLines:  0, // Should default to -1 (all lines)
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := tool.InvokableRun(ctx, string(inputJSON))
	require.NoError(t, err)
	assert.Equal(t, "line 1\nline 2\nline 3\nline 4\nline 5", output)
}

func TestWriteFileTool_MultipleWrites(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tool := NewWriteFileTool(tmpDir)
	ctx := context.Background()

	// First write
	input1 := WriteFileInput{
		Path:    "multiwrite.txt",
		Content: "first content",
	}
	inputJSON1, err := json.Marshal(input1)
	require.NoError(t, err)

	output1, err := tool.InvokableRun(ctx, string(inputJSON1))
	require.NoError(t, err)
	assert.Contains(t, output1, "success")

	// Second write (overwrite)
	input2 := WriteFileInput{
		Path:    "multiwrite.txt",
		Content: "second content",
	}
	inputJSON2, err := json.Marshal(input2)
	require.NoError(t, err)

	output2, err := tool.InvokableRun(ctx, string(inputJSON2))
	require.NoError(t, err)
	assert.Contains(t, output2, "success")

	// Verify final content
	content, err := os.ReadFile(filepath.Join(tmpDir, "multiwrite.txt"))
	require.NoError(t, err)
	assert.Equal(t, "second content", string(content))
}

func TestListDirectoryTool_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "empty_dir_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create an empty subdirectory
	emptyDir := filepath.Join(tmpDir, "empty")
	err = os.Mkdir(emptyDir, 0755)
	require.NoError(t, err)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	input := ListDirectoryInput{
		Path:      "empty",
		Recursive: false,
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	output, err := tool.InvokableRun(ctx, string(inputJSON))
	require.NoError(t, err)
	// Empty directory should return empty string
	assert.Equal(t, "", strings.TrimSpace(output))
}
