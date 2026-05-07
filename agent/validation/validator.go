package validation

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Stage constants for validation phases
const (
	StageTSC   = "tsc"
	StageBuild = "build"
)

// FileError represents a structured error at a specific file location
type FileError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Code    string `json:"code"`    // e.g. "TS2304"
	Message string `json:"message"` // human-readable error text
}

// Result holds the outcome of one validation stage
type Result struct {
	Stage     string      `json:"stage"`
	Passed    bool        `json:"passed"`
	RawOutput string      `json:"raw_output,omitempty"`
	Errors    []FileError `json:"errors,omitempty"`
	Duration  string      `json:"duration"`
}

// tscErrorRe parses TypeScript compiler errors:
//
//	src/App.tsx(10,5): error TS2304: Cannot find name 'foo'.
var tscErrorRe = regexp.MustCompile(`^([^(]+)\((\d+),(\d+)\):\s+error\s+(TS\d+):\s+(.+)$`)

// viteErrorRe parses Vite build errors (simplified):
//
//	[vite]: Rollup failed to resolve import "foo" from "src/App.tsx".
var viteErrorRe = regexp.MustCompile(`(?i)error[:\s]`)

// toDockerMount converts an absolute host path to a Docker-compatible volume path.
func toDockerMount(absPath string) string {
	if runtime.GOOS == "windows" {
		if len(absPath) >= 2 && absPath[1] == ':' {
			driveLetter := strings.ToLower(string(absPath[0]))
			rest := filepath.ToSlash(absPath[2:])
			return "/" + driveLetter + rest
		}
	}
	return filepath.ToSlash(absPath)
}

// runInDocker executes a shell command inside a temporary node:20-alpine container
// with the projectDir mounted at /app. It returns the combined stdout+stderr output.
// timeoutSec caps how long the container is allowed to run.
func runInDocker(absProjectDir string, shellCmd string, timeoutSec int) (string, error) {
	// Always resolve to an absolute path — callers may pass a relative path.
	resolved, err := filepath.Abs(absProjectDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve project dir %q: %w", absProjectDir, err)
	}
	mountPath := toDockerMount(resolved)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/app", mountPath),
		"-w", "/app",
		"node:20-alpine",
		"sh", "-c", shellCmd,
	)

	out, _ := cmd.CombinedOutput()
	output := string(out)
	snippet := output
	if len(snippet) > 500 {
		snippet = snippet[:500]
	}

	var runErr error
	if ctx.Err() != nil {
		runErr = fmt.Errorf("timeout after %ds", timeoutSec)
	} else if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		runErr = fmt.Errorf("exit code %d", cmd.ProcessState.ExitCode())
	}
	log.Printf("[Docker] dir=%s\ncmd=%q\noutput=%s\nerr=%v", resolved, shellCmd[:min(len(shellCmd), 80)], snippet, runErr)
	return output, runErr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseTSCErrors parses tsc --noEmit output into structured FileError list.
func parseTSCErrors(output string) []FileError {
	var errs []FileError
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := tscErrorRe.FindStringSubmatch(line); len(m) == 6 {
			file := strings.TrimSpace(m[1])
			var lineNum, colNum int
			fmt.Sscanf(m[2], "%d", &lineNum)
			fmt.Sscanf(m[3], "%d", &colNum)
			errs = append(errs, FileError{
				File:    file,
				Line:    lineNum,
				Col:     colNum,
				Code:    m[4],
				Message: strings.TrimSpace(m[5]),
			})
		}
	}
	return errs
}

// parseBuildErrors extracts error lines from Vite / Expo build output.
func parseBuildErrors(output string) []FileError {
	var errs []FileError
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if viteErrorRe.MatchString(line) && len(line) > 5 {
			errs = append(errs, FileError{
				File:    "",
				Message: line,
			})
		}
	}
	return errs
}

// RunInstall runs npm install --prefer-offline in the project directory via Docker.
// Call this once before RunTSC/RunBuild so node_modules is populated.
func RunInstall(absProjectDir string) (string, error) {
	return runInDocker(absProjectDir, "npm install --prefer-offline 2>&1", 180)
}

// RunTSC runs TypeScript type-checking (tsc --noEmit) inside Docker.
// It assumes node_modules is already installed in the project directory.
// framework should be "react", "vue3", or "react-native".
func RunTSC(absProjectDir, framework string) *Result {
	start := time.Now()

	// Use the local TypeScript binary from node_modules — avoids npx installing
	// the wrong 'tsc' package (there's a fake npm package called 'tsc@2.0.4').
	tscCmd := "./node_modules/.bin/tsc --noEmit 2>&1"

	output, err := runInDocker(absProjectDir, tscCmd, 60)
	dur := time.Since(start).Round(time.Millisecond).String()

	passed := err == nil && !strings.Contains(output, "error TS")
	errs := parseTSCErrors(output)

	return &Result{
		Stage:     StageTSC,
		Passed:    passed,
		RawOutput: output,
		Errors:    errs,
		Duration:  dur,
	}
}

// RunBuild runs the production build (Vite build / expo export:web) inside Docker.
// It assumes node_modules is already installed.
func RunBuild(absProjectDir, framework string) *Result {
	start := time.Now()

	var buildCmd string
	switch framework {
	case "react-native":
		buildCmd = "./node_modules/.bin/expo export:web 2>&1"
	default:
		buildCmd = "./node_modules/.bin/vite build 2>&1"
	}

	output, err := runInDocker(absProjectDir, buildCmd, 120)
	dur := time.Since(start).Round(time.Millisecond).String()

	passed := err == nil && !strings.Contains(strings.ToLower(output), "error")
	errs := parseBuildErrors(output)

	return &Result{
		Stage:     StageBuild,
		Passed:    passed,
		RawOutput: output,
		Errors:    errs,
		Duration:  dur,
	}
}

// FormatErrorsForLLM returns a compact, LLM-readable summary of validation errors.
func FormatErrorsForLLM(results []*Result) string {
	var sb strings.Builder
	for _, r := range results {
		if r.Passed {
			continue
		}
		sb.WriteString(fmt.Sprintf("=== %s errors ===\n", strings.ToUpper(r.Stage)))
		if len(r.Errors) > 0 {
			for _, e := range r.Errors {
				if e.File != "" {
					sb.WriteString(fmt.Sprintf("  %s(%d,%d): %s %s\n", e.File, e.Line, e.Col, e.Code, e.Message))
				} else {
					sb.WriteString(fmt.Sprintf("  %s\n", e.Message))
				}
			}
		} else {
			// Fallback: include first 1000 chars of raw output
			raw := r.RawOutput
			if len(raw) > 1000 {
				raw = raw[:1000] + "...(truncated)"
			}
			sb.WriteString(raw)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// StageE2E is the stage name for Playwright E2E tests.
const StageE2E = "e2e"

// RunE2E runs Playwright end-to-end tests against a live dev server URL.
// It spins up a temporary node:20-alpine container with @playwright/test installed
// and runs the inline test script against the target URL.
// targetURL should be the base URL of the running dev server (e.g. "http://host.docker.internal:5173").
func RunE2E(targetURL string) *Result {
	start := time.Now()

	// Inline Playwright test: verifies the page loads and has a non-empty body.
	// Using --network=host isn't available on all Docker Desktop setups,
	// so we pass the URL as an env variable and use host.docker.internal for Windows/Mac.
	e2eScript := fmt.Sprintf(`
npx --yes playwright install chromium --with-deps 2>/dev/null || true
node -e "
const { chromium } = require('@playwright/test');
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  try {
    const resp = await page.goto('%s', { timeout: 30000, waitUntil: 'domcontentloaded' });
    const status = resp ? resp.status() : 0;
    const body = await page.evaluate(() => document.body ? document.body.innerText.trim() : '');
    if (status >= 400) { console.error('E2E_FAIL: HTTP ' + status); process.exit(1); }
    if (!body) { console.error('E2E_FAIL: page body is empty'); process.exit(1); }
    console.log('E2E_PASS: page loaded, status=' + status);
    process.exit(0);
  } catch(e) { console.error('E2E_FAIL: ' + e.message); process.exit(1); }
  finally { await browser.close(); }
})();
"
`, targetURL)

	cmd := exec.Command("docker", "run", "--rm",
		"--add-host=host.docker.internal:host-gateway",
		"mcr.microsoft.com/playwright:v1.44.0-jammy",
		"bash", "-c", e2eScript,
	)
	out, err := cmd.CombinedOutput()
	output := string(out)
	dur := time.Since(start).Round(time.Millisecond).String()

	passed := err == nil && strings.Contains(output, "E2E_PASS")

	var errs []FileError
	if !passed {
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "E2E_FAIL") {
				errs = append(errs, FileError{Message: strings.TrimSpace(line)})
			}
		}
		if len(errs) == 0 && !passed {
			errs = append(errs, FileError{Message: "E2E check failed — see raw output"})
		}
	}

	return &Result{
		Stage:     StageE2E,
		Passed:    passed,
		RawOutput: output,
		Errors:    errs,
		Duration:  dur,
	}
}
