//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	ctx := context.Background()
	ctx = agentcontext.InitContextParams(ctx)

	testProjectID := "test-manual-001"
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": testProjectID,
	})

	projectRoot := "./projects"

	fmt.Println("=== Testing Agent Tools ===")
	fmt.Printf("Project ID: %s\n", testProjectID)
	fmt.Printf("Project Root: %s\n\n", projectRoot)

	// Test 1: scaffold_project
	fmt.Println("Test 1: scaffold_project (React)")
	fmt.Println("-----------------------------------")
	scaffoldTool := tools.NewScaffoldProjectTool(projectRoot)
	result, err := scaffoldTool.InvokableRun(ctx, `{"framework":"react"}`)
	if err != nil {
		log.Fatalf("❌ scaffold_project failed: %v", err)
	}
	fmt.Printf("✓ Result: %s\n\n", result)

	// Test 2: write_file
	fmt.Println("Test 2: write_file (src/components/Button.tsx)")
	fmt.Println("-----------------------------------")
	writeTool := tools.NewWriteFileTool(projectRoot)
	buttonCode := `import React from 'react';

interface ButtonProps {
  onClick: () => void;
  children: React.ReactNode;
  variant?: 'primary' | 'secondary';
}

export function Button({ onClick, children, variant = 'primary' }: ButtonProps) {
  return (
    <button
      onClick={onClick}
      className={variant === 'primary' ? 'btn-primary' : 'btn-secondary'}
    >
      {children}
    </button>
  );
}
`
	writeInput := fmt.Sprintf(`{
		"path": "src/components/Button.tsx",
		"content": %q
	}`, buttonCode)

	result, err = writeTool.InvokableRun(ctx, writeInput)
	if err != nil {
		log.Fatalf("❌ write_file failed: %v", err)
	}
	fmt.Printf("✓ Result: %s\n\n", result)

	// Test 3: read_file
	fmt.Println("Test 3: read_file (src/components/Button.tsx)")
	fmt.Println("-----------------------------------")
	readTool := tools.NewReadFileTool(projectRoot)
	result, err = readTool.InvokableRun(ctx, `{"path":"src/components/Button.tsx"}`)
	if err != nil {
		log.Fatalf("❌ read_file failed: %v", err)
	}
	fmt.Printf("✓ File content (first 200 chars):\n%s...\n\n", result[:min(200, len(result))])

	// Test 4: list_directory
	fmt.Println("Test 4: list_directory (src)")
	fmt.Println("-----------------------------------")
	listTool := tools.NewListDirectoryTool(projectRoot)
	result, err = listTool.InvokableRun(ctx, `{"path":"src","recursive":true}`)
	if err != nil {
		log.Fatalf("❌ list_directory failed: %v", err)
	}
	fmt.Printf("✓ Directory structure:\n%s\n\n", result)

	// Verify files exist
	fmt.Println("=== Verification ===")
	fmt.Println("-----------------------------------")
	projectPath := fmt.Sprintf("%s/%s", projectRoot, testProjectID)

	filesToCheck := []string{
		"package.json",
		"tsconfig.json",
		"vite.config.ts",
		"index.html",
		"src/main.tsx",
		"src/App.tsx",
		"src/components/Button.tsx",
	}

	for _, file := range filesToCheck {
		fullPath := fmt.Sprintf("%s/%s", projectPath, file)
		if _, err := os.Stat(fullPath); err == nil {
			fmt.Printf("✓ %s exists\n", file)
		} else {
			fmt.Printf("❌ %s NOT FOUND\n", file)
		}
	}

	fmt.Println("\n=== All Tests Completed ===")
	fmt.Printf("Project created at: %s\n", projectPath)
	fmt.Println("\nTo view the project:")
	fmt.Printf("  cd %s\n", projectPath)
	fmt.Println("  npm install")
	fmt.Println("  npm run dev")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
