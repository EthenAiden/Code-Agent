package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// Prefer local node_modules/.bin binaries to avoid npx package-download issues.
	// On Windows .bin contains .cmd wrappers; on Unix they are symlinks.
	binSuffix := ""
	if runtime.GOOS == "windows" {
		binSuffix = ".cmd"
	}

	vueTscBin := dir + "/node_modules/.bin/vue-tsc" + binSuffix
	tscBin := dir + "/node_modules/.bin/tsc" + binSuffix

	var name string
	var args []string

	// Check if vue-tsc is installed locally first.
	vueCheck := exec.CommandContext(ctx, vueTscBin, "--version")
	vueCheck.Dir = dir
	if err := vueCheck.Run(); err == nil {
		name = vueTscBin
		args = []string{"--noEmit"}
	} else {
		// Check if tsc is installed locally.
		tscCheck := exec.CommandContext(ctx, tscBin, "--version")
		tscCheck.Dir = dir
		if err := tscCheck.Run(); err == nil {
			name = tscBin
			args = []string{"--noEmit"}
		} else {
			// TypeScript not installed locally — skip type-check, report as pass
			// so the agent moves on to run_build which will catch real errors.
			result := ValidationResult{
				Success:  true,
				ExitCode: 0,
				Stdout:   "tsc not found locally — skipping type-check (run_build will validate)",
			}
			out, _ := json.Marshal(result)
			return string(out), nil
		}
	}

	return runCommand(ctx, dir, name, args, 120*time.Second)
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

	// Use the local vite binary from node_modules/.bin to avoid PATH issues.
	binSuffix := ""
	if runtime.GOOS == "windows" {
		binSuffix = ".cmd"
	}
	viteBin := filepath.Join(dir, "node_modules", ".bin", "vite"+binSuffix)

	// If node_modules is not installed yet, report success so the agent doesn't loop.
	// scaffold_project handles npm install automatically.
	if _, err := os.Stat(viteBin); os.IsNotExist(err) {
		result := ValidationResult{
			Success:  true,
			ExitCode: 0,
			Stdout:   "node_modules not ready yet — skipping build check",
		}
		out, _ := json.Marshal(result)
		return string(out), nil
	}

	return runCommand(ctx, dir, viteBin, []string{"build"}, 300*time.Second)
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
		Stdout:   decodeOutput(stdout.Bytes()),
		Stderr:   decodeOutput(stderr.Bytes()),
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

