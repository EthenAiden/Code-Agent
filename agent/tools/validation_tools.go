package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ─── run_type_check tool ──────────────────────────────────────────────────────

var runTypeCheckToolInfo = &schema.ToolInfo{
	Name: "run_type_check",
	Desc: `Run TypeScript type checking on the project using tsc or vue-tsc (for Vue 3).
Use this after writing TypeScript/Vue files to catch type errors before running the build.
Returns: exit code, stdout, stderr.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
}

type runTypeCheckTool struct {
	baseRoot string
}

// NewRunTypeCheckTool creates the run_type_check tool.
func NewRunTypeCheckTool(baseRoot string) tool.InvokableTool {
	return &runTypeCheckTool{baseRoot: baseRoot}
}

func (t *runTypeCheckTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return runTypeCheckToolInfo, nil
}

func (t *runTypeCheckTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	dir := projectDir(ctx, t.baseRoot)

	// Detect framework to pick the right type-check command.
	// vue3 → vue-tsc --noEmit, react / react-native → tsc --noEmit
	tscCmd := "tsc"
	tscArgs := []string{"--noEmit"}

	// Check if vue-tsc is available (present in node_modules/.bin)
	vueCheck := exec.CommandContext(ctx, "npx", "--no-install", "vue-tsc", "--version")
	vueCheck.Dir = dir
	if err := vueCheck.Run(); err == nil {
		tscCmd = "npx"
		tscArgs = []string{"vue-tsc", "--noEmit"}
	} else {
		tscCmd = "npx"
		tscArgs = []string{"tsc", "--noEmit"}
	}

	return runCommand(ctx, dir, tscCmd, tscArgs, 120*time.Second)
}

// ─── run_build tool ──────────────────────────────────────────────────────────

var runBuildToolInfo = &schema.ToolInfo{
	Name: "run_build",
	Desc: `Run Vite build (npm run build) to validate that the project compiles and bundles successfully.
Use this as a final validation step after all code has been written and type-checked.
Returns: exit code, stdout, stderr.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
}

type runBuildTool struct {
	baseRoot string
}

// NewRunBuildTool creates the run_build tool.
func NewRunBuildTool(baseRoot string) tool.InvokableTool {
	return &runBuildTool{baseRoot: baseRoot}
}

func (t *runBuildTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return runBuildToolInfo, nil
}

func (t *runBuildTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	dir := projectDir(ctx, t.baseRoot)
	return runCommand(ctx, dir, "npm", []string{"run", "build"}, 300*time.Second)
}

// ─── shared helper ───────────────────────────────────────────────────────────

// ValidationResult is the structured output returned by both validation tools.
type ValidationResult struct {
	Success  bool   `json:"success"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

// runCommand executes a command in dir and returns a JSON-encoded ValidationResult.
func runCommand(ctx context.Context, dir, name string, args []string, timeout time.Duration) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ValidationResult{
		Success:  err == nil,
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	out, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return fmt.Sprintf("exit_code=%d\nstdout=%s\nstderr=%s", result.ExitCode, result.Stdout, result.Stderr), nil
	}
	return string(out), nil
}
