# Agent Tools

This package provides tools for the Code-Agent system, enabling agents to perform file operations and execute code within project directories.

## Tools

### ReadFileTool

Reads content from files in the project directory.

**Parameters:**
- `path` (string, required): File path relative to project root
- `start_line` (int, optional): Starting line number (1-indexed), defaults to 1
- `num_lines` (int, optional): Number of lines to read, -1 means read all remaining lines, defaults to -1

**Example:**
```json
{
  "path": "src/main.go",
  "start_line": 10,
  "num_lines": 20
}
```

**Security:**
- Validates paths to prevent directory traversal attacks
- Restricts access to project directory only
- Returns descriptive error messages for invalid paths

### WriteFileTool

Writes content to files in the project directory.

**Parameters:**
- `path` (string, required): File path relative to project root
- `content` (string, required): Content to write to the file

**Example:**
```json
{
  "path": "src/newfile.go",
  "content": "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"
}
```

**Security:**
- Validates paths to prevent directory traversal attacks
- Blocks writes to sensitive files (.env, .git, .ssh, private keys)
- Restricts access to project directory only
- Creates parent directories automatically if needed
- Overwrites existing files (use with caution)

### ListDirectoryTool

Lists contents of directories in the project.

**Parameters:**
- `path` (string, required): Directory path relative to project root
- `recursive` (bool, optional): Whether to list subdirectories recursively, defaults to false

**Example:**
```json
{
  "path": "src",
  "recursive": true
}
```

**Security:**
- Validates paths to prevent directory traversal attacks
- Restricts access to project directory only
- Returns file/directory type information

### ExecuteCodeTool

Executes code in a sandboxed environment with timeout and resource limits.

**Parameters:**
- `language` (string, required): Programming language (python, javascript, go)
- `code` (string, required): Code to execute
- `timeout` (int, optional): Maximum execution time in seconds (default: 30, max: 300)

**Supported Languages:**
- `python`: Executes Python code using python/python3
- `javascript`: Executes JavaScript code using node
- `go`: Executes Go code using go run

**Example:**
```json
{
  "language": "python",
  "code": "print('Hello, World!')",
  "timeout": 10
}
```

**Security:**
- Executes code in isolated temporary directories
- Enforces timeout limits (default 30s, max 300s)
- Captures both stdout and stderr
- Automatically detects and uses available Python interpreter (python3 or python)
- Cleans up temporary files after execution

**Resource Limits:**
- Timeout enforcement via context cancellation
- Isolated execution environment (temporary directory per execution)
- No network access (depends on system configuration)
- Limited to configured timeout duration

### GetProjectContextTool

Retrieves project context including metadata, file structure, and existing code.

**Parameters:**
- `project_id` (string, required): Project ID to retrieve context for
- `user_id` (string, required): User ID who owns the project
- `include_files` (bool, optional): Whether to include file structure, defaults to true
- `max_depth` (int, optional): Maximum directory depth for file structure (default: 3, max: 10)

**Example:**
```json
{
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "include_files": true,
  "max_depth": 3
}
```

**Output:**
```json
{
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "message_count": 5,
  "created_at": "2024-01-15 10:30:00",
  "file_structure": {
    "name": "project-root",
    "type": "directory",
    "path": "",
    "children": [...]
  },
  "summary": "Project 550e8400-e29b-41d4-a716-446655440000 has 5 messages. Created at 2024-01-15 10:30:00."
}
```

**Features:**
- Retrieves project metadata from ProjectManager service
- Builds hierarchical file structure tree
- Respects max depth limit to avoid excessive recursion
- Skips common ignore patterns (node_modules, .git, vendor, etc.)
- Provides project summary with message count and creation time

**Security:**
- Integrates with existing ProjectManager authentication
- Validates project ownership before returning data
- Skips hidden files and sensitive directories

## Usage

```go
import (
    "github.com/ethen-aiden/code-agent/agent/tools"
    "github.com/ethen-aiden/code-agent/service"
)

// Initialize tools with project root
projectRoot := "/path/to/project"
readTool := tools.NewReadFileTool(projectRoot)
writeTool := tools.NewWriteFileTool(projectRoot)
listTool := tools.NewListDirectoryTool(projectRoot)
execTool := tools.NewExecuteCodeTool(projectRoot)

// Initialize project context tool with ProjectManager
projectManager := service.NewProjectManager(sessionRepo, cacheRepo)
projectTool := tools.NewGetProjectContextTool(projectManager, projectRoot)

// Use tools in agent context
ctx := context.Background()

// Read file
inputJSON := `{"path": "README.md", "start_line": 1, "num_lines": 10}`
output, err := readTool.InvokableRun(ctx, inputJSON)

// Execute code
codeJSON := `{"language": "python", "code": "print('Hello')", "timeout": 5}`
result, err := execTool.InvokableRun(ctx, codeJSON)

// Get project context
projectJSON := `{"project_id": "550e8400-e29b-41d4-a716-446655440000", "user_id": "123e4567-e89b-12d3-a456-426614174000", "include_files": true, "max_depth": 3}`
context, err := projectTool.InvokableRun(ctx, projectJSON)
```

## Security Features

All tools implement the following security measures:

1. **Path Validation**: Prevents directory traversal attacks using `../` patterns
2. **Project Root Restriction**: All file operations are restricted to the project directory
3. **Sensitive File Protection**: WriteFileTool blocks writes to sensitive files and directories
4. **Code Execution Sandboxing**: ExecuteCodeTool runs code in isolated temporary directories
5. **Timeout Enforcement**: Code execution is limited by configurable timeouts
6. **Error Handling**: Returns descriptive error messages without exposing system details

## Testing

Run tests with:
```bash
go test -v ./agent/tools/
```

Tests cover:
- Successful file operations
- Code execution for Python, JavaScript, and Go
- Timeout enforcement for long-running code
- Error handling for invalid inputs
- Security validation (directory traversal, sensitive files)
- Edge cases (empty directories, multiple writes, default values)
- Cross-platform Python interpreter detection

## Requirements Mapping

This implementation satisfies the following requirements:

- **7.1**: Tool system with file operations, code execution, and project context
- **7.2**: File reading operations
- **7.3**: File writing operations
- **7.4**: Code execution in sandboxed environment
- **7.5**: Tool descriptions for agent understanding
- **7.6**: Descriptive error messages
- **7.7**: Code execution with resource limits (timeout)
- **7.8**: File operation restrictions to project directory
- **7.9**: Path validation to prevent directory traversal attacks
- **11.2**: Project metadata retrieval
- **11.3**: Project file structure and existing code access
