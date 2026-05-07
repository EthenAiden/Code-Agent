package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	baseDir string
	once    sync.Once
)

func initBaseDir() {
	// Production: look for prompts/ next to the executable
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "prompts")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			baseDir = candidate
			return
		}
	}

	// Development fallback: use the source-file directory (agent-server/prompts/)
	_, file, _, ok := runtime.Caller(0)
	if ok {
		baseDir = filepath.Dir(file)
		return
	}

	// Last resort: cwd/prompts
	cwd, _ := os.Getwd()
	baseDir = filepath.Join(cwd, "prompts")
}

// Load reads a prompt file by name from the prompts directory.
// It panics on failure so misconfigured prompts are caught at startup.
func Load(name string) string {
	once.Do(initBaseDir)
	data, err := os.ReadFile(filepath.Join(baseDir, name))
	if err != nil {
		panic(fmt.Sprintf("prompts: failed to load %q from %q: %v", name, baseDir, err))
	}
	return string(data)
}
