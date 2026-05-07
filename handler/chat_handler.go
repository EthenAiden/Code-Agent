package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/agent/sequential"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/agent/validation"
	agentmodel "github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/prompts"
	"github.com/ethen-aiden/code-agent/service"
)

// ChatHandler handles chat endpoints with session management
type ChatHandler struct {
	messageHistoryService *service.MessageHistoryService
	projectManager        tools.ProjectManagerInterface
	buildHandler          *BuildHandler
	sequentialAgent       *sequential.SequentialAgent
	chatModel             einomodel.ToolCallingChatModel
	projectRoot           string
}

// NewChatHandler creates a new ChatHandler with dependency injection
func NewChatHandler(
	messageHistoryService *service.MessageHistoryService,
	projectManager tools.ProjectManagerInterface,
	buildHandler *BuildHandler,
	sequentialAgent *sequential.SequentialAgent,
	chatModel einomodel.ToolCallingChatModel,
	projectRoot string,
) *ChatHandler {
	return &ChatHandler{
		messageHistoryService: messageHistoryService,
		projectManager:        projectManager,
		buildHandler:          buildHandler,
		sequentialAgent:       sequentialAgent,
		chatModel:             chatModel,
		projectRoot:           projectRoot,
	}
}

// generatedFile holds a single file parsed from the model's XML output
type generatedFile struct {
	Path    string
	Content string
}

// fileBlockRe matches <file path="...">...</file> blocks (non-greedy, dot-all)
var fileBlockRe = regexp.MustCompile(`(?s)<file\s+path="([^"]+)"\s*>(.*?)</file>`)

// parseFileBlocks extracts all <file path="..."> blocks from the model output
func parseFileBlocks(output string) []generatedFile {
	matches := fileBlockRe.FindAllStringSubmatch(output, -1)
	files := make([]generatedFile, 0, len(matches))
	for _, m := range matches {
		content := strings.TrimSpace(m[2])
		// Strip wrapping markdown code fences if present
		if idx := strings.Index(content, "\n"); idx != -1 {
			firstLine := strings.TrimSpace(content[:idx])
			if strings.HasPrefix(firstLine, "```") {
				content = content[idx+1:]
				if last := strings.LastIndex(content, "```"); last != -1 {
					content = strings.TrimSpace(content[:last])
				}
			}
		}
		files = append(files, generatedFile{Path: m[1], Content: content})
	}
	return files
}

// writeProjectFile writes content to projectRoot/projectID/relPath, creating dirs as needed
func (h *ChatHandler) writeProjectFile(projectID, relPath, content string) error {
	full := filepath.Join(h.projectRoot, projectID, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0644)
}

// projectHasFiles returns true if the project workspace already has any files
func (h *ChatHandler) projectHasFiles(projectID string) bool {
	dir := filepath.Join(h.projectRoot, projectID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			return true
		}
	}
	return false
}

// detectFramework reads package.json in the project dir and returns the framework string.
// Returns "react" as default if detection fails.
func (h *ChatHandler) detectFramework(projectID string) string {
	pkgPath := filepath.Join(h.projectRoot, projectID, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return "react"
	}
	content := string(data)
	if strings.Contains(content, "\"expo\"") || strings.Contains(content, "\"react-native\"") {
		return "react-native"
	}
	if strings.Contains(content, "\"vue\"") {
		return "vue3"
	}
	return "react"
}

// autoScaffold writes the framework boilerplate directly without going through the agent
func (h *ChatHandler) autoScaffold(projectID, framework string) error {
	spec := tools.GetFrameworkSpec(framework)
	if spec == nil {
		return fmt.Errorf("framework spec not found: %s", framework)
	}

	files := map[string]string{
		"package.json":     spec.PackageJSON,
		"tsconfig.json":    spec.TsConfig,
		"vite.config.ts":   spec.ViteConfig,
		"index.html":       spec.IndexHTML,
		spec.EntryFileName: spec.EntryContent,
		spec.AppFileName:   spec.AppContent,
	}
	for k, v := range spec.ExtraFiles {
		files[k] = v
	}

	for relPath, content := range files {
		if content == "" {
			continue
		}
		if err := h.writeProjectFile(projectID, relPath, content); err != nil {
			return fmt.Errorf("scaffold %s: %w", relPath, err)
		}
	}
	return nil
}

// buildSystemPrompt returns the Lovable-style code generation system prompt
func buildSystemPrompt(framework, existingFiles string) string {
	frameworkConstraints := tools.GetFrameworkPromptConstraints(framework)

	existingSection := ""
	if existingFiles != "" {
		existingSection = fmt.Sprintf(`
## Existing Project Files
The project already has these files. Modify them when needed; don't recreate unchanged files.
%s
`, existingFiles)
	}

	return fmt.Sprintf(prompts.Load("system_chat_handler.txt"), frameworkConstraints, existingSection)
}

// listExistingFiles returns a simple listing of files already in the project workspace
func (h *ChatHandler) listExistingFiles(projectID string) string {
	dir := filepath.Join(h.projectRoot, projectID)
	var lines []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "node_modules/") || strings.HasPrefix(rel, "dist/") {
			return nil
		}
		lines = append(lines, "- "+rel)
		return nil
	})
	return strings.Join(lines, "\n")
}

// buildErrorFileContext reads the files mentioned in validation errors and
// returns their full contents + the actual src/ file listing so the model
// can resolve filename casing mismatches (TS1261).
func (h *ChatHandler) buildErrorFileContext(absProjectDir string, results []*validation.Result) string {
	seen := make(map[string]bool)
	var filePaths []string
	for _, r := range results {
		for _, e := range r.Errors {
			if e.File == "" || seen[e.File] {
				continue
			}
			seen[e.File] = true
			filePaths = append(filePaths, e.File)
		}
	}

	var sb strings.Builder

	// Always include the actual file listing so the model can resolve casing
	// mismatches (TS1261: file differs only in casing).
	sb.WriteString("## Actual files on disk (use these exact names for imports)\n")
	_ = filepath.Walk(absProjectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(absProjectDir, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "node_modules/") || strings.HasPrefix(rel, "dist/") {
			return nil
		}
		sb.WriteString("- " + rel + "\n")
		return nil
	})
	sb.WriteString("\n")

	if len(filePaths) == 0 {
		return sb.String()
	}

	sb.WriteString("## File contents\n")
	for _, rel := range filePaths {
		abs := filepath.Join(absProjectDir, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("<file path=%q>\n%s\n</file>\n\n", rel, string(data)))
	}
	return sb.String()
}

// ── SSE trace helper ──────────────────────────────────────────────────────────

// traceWriter wraps an SSE writer and records every agent_event for persistence.
type traceWriter struct {
	w      *sse.Writer
	events []json.RawMessage
}

// emit sends an agent_event over SSE and appends it to the trace.
func (t *traceWriter) emit(payload []byte) {
	_ = t.w.WriteEvent("", "agent_event", payload)
	cp := make(json.RawMessage, len(payload))
	copy(cp, payload)
	t.events = append(t.events, cp)
}

// metadataJSON returns the trace as a JSON array string, or "" on error.
func (t *traceWriter) metadataJSON() string {
	if len(t.events) == 0 {
		return "[]"
	}
	b, err := json.Marshal(t.events)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ── Chat HTTP handler ─────────────────────────────────────────────────────────

// Chat handles POST /api/v1/projects/{project_id}/chat
func (h *ChatHandler) Chat(ctx context.Context, c *app.RequestContext) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "project_id path parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	var req agentmodel.ChatRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(http.StatusBadRequest, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   fmt.Sprintf("Invalid request body: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	if err := h.messageHistoryService.CreateSessionIfNotExists(ctx, projectID, userIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "SESSION_CREATION_FAILED",
				Message:   fmt.Sprintf("Failed to create session: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	ctx = agentcontext.InitContextParams(ctx)
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": projectID,
		"user_id":    userIDStr,
		"session_id": projectID,
	})

	userMsg := agentmodel.Message{
		ConversationID: projectID,
		Role:           "user",
		Content:        req.Message,
		Timestamp:      time.Now(),
	}
	if err := h.messageHistoryService.InsertMessage(ctx, userMsg, userIDStr); err != nil {
		log.Printf("Warning: failed to store user message: %v", err)
	}

	history, err := h.messageHistoryService.GetMessageHistory(ctx, projectID, userIDStr)
	if err != nil {
		log.Printf("Warning: failed to get message history: %v", err)
	}

	isFirstTurn := len(history) <= 1

	w := sse.NewWriter(c)
	defer w.Close()

	if err := w.WriteEvent("", "ping", []byte("")); err != nil {
		log.Printf("Failed to write ping: %v", err)
		return
	}

	h.handleCodeGeneration(ctx, w, projectID, userIDStr, req.Message, history, req, isFirstTurn)
}

// ── handleChatReply ───────────────────────────────────────────────────────────

// handleChatReply sends a streaming conversational response via the chat model.
func (h *ChatHandler) handleChatReply(
	ctx context.Context,
	w *sse.Writer,
	projectID, userIDStr, userMessage string,
	history []agentmodel.Message,
) {
	messages := buildSchemaMessages(history, userMessage)

	stream, err := h.chatModel.Stream(ctx, messages)
	if err != nil {
		_ = w.WriteEvent("", "error", []byte(err.Error()))
		return
	}

	var fullResponse strings.Builder
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		if msg != nil && msg.Content != "" {
			fullResponse.WriteString(msg.Content)
			_ = w.WriteEvent("", "message", []byte(msg.Content))
		}
	}

	_ = w.WriteEvent("", "done", []byte(""))

	_ = h.messageHistoryService.InsertMessage(ctx, agentmodel.Message{
		ConversationID: projectID,
		Role:           "assistant",
		Content:        fullResponse.String(),
		Timestamp:      time.Now(),
	}, userIDStr)
}

// ── handleCodeGeneration (dispatcher) ────────────────────────────────────────

// handleCodeGeneration runs the full closed-loop generation pipeline:
//  1. Determine framework
//  2. Send plan event
//  3. Scaffold if new project
//  4. Generate code (executor model)
//  5. Validation + self-repair loop (≤3 rounds)
//  6. Persist assistant message with execution trace in metadata
//  7. Start dev server
func (h *ChatHandler) handleCodeGeneration(
	ctx context.Context,
	w *sse.Writer,
	projectID, userIDStr, userMessage string,
	history []agentmodel.Message,
	req agentmodel.ChatRequest,
	isFirstTurn bool,
) {
	tw := &traceWriter{w: w}
	isNewProject := !h.projectHasFiles(projectID)

	// Determine framework
	framework := req.Framework
	if framework == "" || !isNewProject {
		framework = h.detectFramework(projectID)
	}
	if framework == "" {
		framework = "react"
	}

	// Inject context keys consumed by planner / executor
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"framework":  framework,
		"project_id": projectID,
		"project_context": map[string]interface{}{
			"existing_files": h.listExistingFiles(projectID),
			"is_new_project": isNewProject,
		},
		"conversation_history": buildHistoryString(history),
	})

	// Build ADK input from history + current user message.
	// History gives the planner "what was built before"; executor never sees it.
	adkMsgs := make([]adk.Message, 0, len(history)+1)
	for _, m := range history {
		if m.Role == "user" || m.Role == "assistant" {
			adkMsgs = append(adkMsgs, &schema.Message{
				Role:    schema.RoleType(m.Role),
				Content: m.Content,
			})
		}
	}
	adkMsgs = append(adkMsgs, &schema.Message{
		Role:    schema.User,
		Content: userMessage,
	})

	iter := h.sequentialAgent.Run(ctx, &adk.AgentInput{
		Messages:        adkMsgs,
		EnableStreaming: true,
	})

	res := consumeAgentEvents(iter, tw, w)
	if res.hasError {
		return
	}

	summary := res.summary
	if summary == "" && len(res.writtenPaths) > 0 {
		summary = fmt.Sprintf("Generated %d files: %s",
			len(res.writtenPaths), strings.Join(res.writtenPaths, ", "))
	}
	if summary == "" {
		summary = "Done"
	}
	_ = w.WriteEvent("", "message", []byte(summary))

	// Persist assistant message with execution trace
	_ = h.messageHistoryService.InsertMessage(ctx, agentmodel.Message{
		ConversationID: projectID,
		Role:           "assistant",
		Content:        summary,
		Timestamp:      time.Now(),
		Metadata:       tw.metadataJSON(),
	}, userIDStr)

	// Start dev server async
	buildEvt, _ := json.Marshal(map[string]interface{}{"type": "status", "message": "Starting dev server..."})
	tw.emit(buildEvt)
	_ = w.WriteEvent("", "done", []byte(""))

	// After the first generation turn is fully complete, generate the project name from the user's message.
	if isFirstTurn {
		go h.generateProjectName(context.Background(), projectID, userIDStr, userMessage)
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		projectDir := filepath.Join(h.projectRoot, projectID)
		h.buildHandler.StartDevServer(projectID, projectDir, userIDStr, framework)
	}()

	// React Native: push to Expo Snack
	if framework == "react-native" {
		absProjectDir := filepath.Join(h.projectRoot, projectID)
		go h.pushToSnack(w, absProjectDir, projectID)
	}
}


// ── runScaffold ───────────────────────────────────────────────────────────────

func (h *ChatHandler) runScaffold(ctx context.Context, tw *traceWriter, projectID, userIDStr, framework string) error {
	statusEvt, _ := json.Marshal(map[string]interface{}{
		"type":    "status",
		"message": fmt.Sprintf("Initializing %s project...", frameworkDisplayName(framework)),
	})
	tw.emit(statusEvt)

	if err := h.autoScaffold(projectID, framework); err != nil {
		return err
	}

	_ = h.projectManager.SetFramework(ctx, projectID, userIDStr, framework)

	doneEvt, _ := json.Marshal(map[string]interface{}{
		"type":    "tool_call",
		"tool":    "scaffold_project",
		"message": fmt.Sprintf("%s project initialized", frameworkDisplayName(framework)),
	})
	tw.emit(doneEvt)
	return nil
}

// ── runGeneration ─────────────────────────────────────────────────────────────

// runGeneration calls the executor model, writes <file> blocks to disk, and
// handles the App.tsx fallback. Returns (writtenPaths, summary, ok).
func (h *ChatHandler) runGeneration(
	ctx context.Context,
	tw *traceWriter,
	w *sse.Writer,
	projectID, userIDStr, userMessage string,
	history []agentmodel.Message,
	framework string,
) ([]string, string, bool) {
	existingFiles := h.listExistingFiles(projectID)
	systemPrompt := buildSystemPrompt(framework, existingFiles)

	callExecutor := h.makeExecutorCaller(ctx, systemPrompt, history, userMessage)
	writeFiles := h.makeFileWriter(tw, projectID)

	genEvt, _ := json.Marshal(map[string]interface{}{"type": "status", "message": "Generating code..."})
	tw.emit(genEvt)

	output, err := callExecutor("")
	if err != nil {
		log.Printf("[CodeGen] Executor error: %v", err)
		_ = w.WriteEvent("", "error", []byte("Code generation failed: "+err.Error()))
		return nil, "", false
	}
	log.Printf("[CodeGen] Raw output length: %d chars", len(output))

	if len(parseFileBlocks(output)) == 0 {
		log.Printf("[CodeGen] No <file> blocks found, streaming as plain message")
		_ = w.WriteEvent("", "message", []byte(output))
		_ = w.WriteEvent("", "done", []byte(""))
		_ = h.messageHistoryService.InsertMessage(ctx, agentmodel.Message{
			ConversationID: projectID, Role: "assistant", Content: output, Timestamp: time.Now(),
		}, userIDStr)
		return nil, "", false
	}

	writtenPaths, summary := writeFiles(output, false)

	// Fallback: if pages were written but App.tsx was not, request it separately
	appWasWritten := false
	hasPages := false
	for _, p := range writtenPaths {
		if p == "src/App.tsx" {
			appWasWritten = true
		}
		if strings.HasPrefix(p, "src/pages/") {
			hasPages = true
		}
	}
	if hasPages && !appWasWritten {
		log.Printf("[CodeGen] Pages present but App.tsx missing — requesting it")
		appPrompt := "The page components were generated but src/App.tsx was not output. " +
			"Output ONLY src/App.tsx using BrowserRouter + Routes to wire all the pages that were just created."
		if appOut, appErr := callExecutor(appPrompt); appErr == nil {
			newPaths, _ := writeFiles(appOut, false)
			writtenPaths = append(writtenPaths, newPaths...)
		}
	}

	return writtenPaths, summary, true
}

// ── runValidationLoop ─────────────────────────────────────────────────────────

// runValidationLoop runs TSC + Build checks, self-repairing up to maxRepairRounds times.
func (h *ChatHandler) runValidationLoop(
	ctx context.Context,
	tw *traceWriter,
	projectID, framework string,
	writtenPaths *[]string,
	summary *string,
) {
	absProjectDir := filepath.Join(h.projectRoot, projectID)

	installEvt, _ := json.Marshal(map[string]interface{}{
		"type": "status", "message": "Installing dependencies...",
	})
	tw.emit(installEvt)
	if out, err := validation.RunInstall(absProjectDir); err != nil {
		log.Printf("[Validation] npm install failed: %v\n%s", err, out)
	}

	// Build messages for repair calls using the same executor model
	existingFiles := h.listExistingFiles(projectID)
	systemPrompt := buildSystemPrompt(framework, existingFiles)
	callRepair := h.makeExecutorCaller(ctx, systemPrompt, nil, "")
	writeFiles := h.makeFileWriter(tw, projectID)

	const maxRepairRounds = 3
	for round := 0; round < maxRepairRounds; round++ {
		tscEvt, _ := json.Marshal(map[string]interface{}{
			"type": "status", "message": fmt.Sprintf("TypeScript check (round %d)...", round+1),
		})
		tw.emit(tscEvt)

		tscResult := validation.RunTSC(absProjectDir, framework)
		rawSnippet := tscResult.RawOutput
		if len(rawSnippet) > 800 {
			rawSnippet = rawSnippet[:800]
		}
		log.Printf("[Validation] TSC round %d: passed=%v\nraw=%s", round+1, tscResult.Passed, rawSnippet)

		var failedResults []*validation.Result
		if !tscResult.Passed {
			failedResults = append(failedResults, tscResult)
		} else {
			buildEvt, _ := json.Marshal(map[string]interface{}{
				"type": "status", "message": fmt.Sprintf("Build check (round %d)...", round+1),
			})
			tw.emit(buildEvt)

			buildResult := validation.RunBuild(absProjectDir, framework)
			log.Printf("[Validation] Build round %d: passed=%v", round+1, buildResult.Passed)
			if !buildResult.Passed {
				failedResults = append(failedResults, buildResult)
			}
		}

		if len(failedResults) == 0 {
			passEvt, _ := json.Marshal(map[string]interface{}{
				"type": "validation_pass", "message": "All checks passed",
			})
			tw.emit(passEvt)
			break
		}

		errSummary := validation.FormatErrorsForLLM(failedResults)
		valErrEvt, _ := json.Marshal(map[string]interface{}{
			"type": "validation_error", "round": round + 1, "message": errSummary,
		})
		tw.emit(valErrEvt)

		if round == maxRepairRounds-1 {
			log.Printf("[Validation] Max repair rounds reached, giving up")
			break
		}

		repairEvt, _ := json.Marshal(map[string]interface{}{
			"type": "status", "message": fmt.Sprintf("Auto-repair round %d/%d...", round+1, maxRepairRounds),
		})
		tw.emit(repairEvt)

		fileContents := h.buildErrorFileContext(absProjectDir, failedResults)
		repairPrompt := fmt.Sprintf(`
You are fixing TypeScript/build errors in a generated frontend project.

## STRICT RULES

1. NEVER simplify or stub out existing components. Keep ALL existing JSX, logic, and routing intact.
   - BAD: replacing a full page with <div>Hello React!</div>
   - BAD: removing BrowserRouter/Routes to avoid an import error

2. For "Cannot find module 'X'" errors:
   - BAD: deleting the import
   - GOOD: output a fixed package.json adding X to dependencies, keep source files unchanged

3. Output ONLY files that actually need to change, using <file> blocks.

4. src/App.tsx must always keep BrowserRouter + Routes if the project uses routing.

## Errors
%s

## Current file contents
%s
`, errSummary, fileContents)

		repairOutput, repairErr := callRepair(repairPrompt)
		if repairErr != nil {
			log.Printf("[Repair] Round %d error: %v", round+1, repairErr)
			break
		}

		newPaths, newSummary := writeFiles(repairOutput, true)
		if len(newPaths) > 0 {
			*writtenPaths = newPaths
			*summary = newSummary
		}

		// Re-install if package.json was touched
		for _, p := range newPaths {
			if p == "package.json" {
				reinstallEvt, _ := json.Marshal(map[string]interface{}{
					"type": "status", "message": "Dependencies changed, reinstalling...",
				})
				tw.emit(reinstallEvt)
				if out, err := validation.RunInstall(absProjectDir); err != nil {
					log.Printf("[Repair] npm install failed: %v\n%s", err, out)
				}
				break
			}
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// makeExecutorCaller returns a closure that calls the executor model with a fixed
// system prompt + history, appending an optional extra user message.
func (h *ChatHandler) makeExecutorCaller(
	ctx context.Context,
	systemPrompt string,
	history []agentmodel.Message,
	userMessage string,
) func(extra string) (string, error) {
	return func(extra string) (string, error) {
		msgs := make([]*schema.Message, 0, len(history)+3)
		msgs = append(msgs, schema.SystemMessage(systemPrompt))

		start := 0
		if len(history) > 6 {
			start = len(history) - 6
		}
		for _, msg := range history[start:] {
			if msg.Role == "user" {
				msgs = append(msgs, schema.UserMessage(msg.Content))
			} else if msg.Role == "assistant" {
				msgs = append(msgs, schema.AssistantMessage(msg.Content, nil))
			}
		}
		if userMessage != "" {
			msgs = append(msgs, schema.UserMessage(userMessage))
		}
		if extra != "" {
			msgs = append(msgs, schema.UserMessage(extra))
		}

		stream, err := h.chatModel.Stream(ctx, msgs)
		if err != nil {
			return "", err
		}
		var buf strings.Builder
		for {
			msg, err := stream.Recv()
			if err != nil {
				break
			}
			if msg != nil && msg.Content != "" {
				buf.WriteString(msg.Content)
			}
		}
		return buf.String(), nil
	}
}

// makeFileWriter returns a closure that parses <file> blocks, writes them to disk,
// and emits tool_call or repair_file SSE events via tw.
func (h *ChatHandler) makeFileWriter(tw *traceWriter, projectID string) func(output string, isRepair bool) ([]string, string) {
	return func(output string, isRepair bool) ([]string, string) {
		generatedFiles := parseFileBlocks(output)
		var writtenPaths []string
		for _, f := range generatedFiles {
			if err := h.writeProjectFile(projectID, f.Path, f.Content); err != nil {
				log.Printf("[CodeGen] Failed to write %s: %v", f.Path, err)
				continue
			}
			writtenPaths = append(writtenPaths, f.Path)
			var evt []byte
			if isRepair {
				evt, _ = json.Marshal(map[string]interface{}{
					"type":    "repair_file",
					"path":    f.Path,
					"message": fmt.Sprintf("Repaired: %s", f.Path),
				})
			} else {
				evt, _ = json.Marshal(map[string]interface{}{
					"type":    "tool_call",
					"tool":    "write_file",
					"args":    fmt.Sprintf(`{"path": "%s"}`, f.Path),
					"message": fmt.Sprintf("Written: %s", f.Path),
				})
			}
			tw.emit(evt)
			log.Printf("[CodeGen] Written: %s", f.Path)
		}
		summary := strings.TrimSpace(fileBlockRe.ReplaceAllString(output, ""))
		if summary == "" && len(writtenPaths) > 0 {
			summary = fmt.Sprintf("Generated %d files: %s", len(writtenPaths), strings.Join(writtenPaths, ", "))
		}
		return writtenPaths, summary
	}
}

// pushToSnack pushes a React Native project to Expo Snack and emits the preview URL.
func (h *ChatHandler) pushToSnack(w *sse.Writer, absProjectDir, projectID string) {
	snackURL, err := tools.PushToSnack(absProjectDir, projectID)
	if err != nil {
		log.Printf("[Snack] Push failed: %v", err)
		errEvt, _ := json.Marshal(map[string]interface{}{
			"type": "status", "message": "Expo Snack push failed, using local Expo Web preview",
		})
		_ = w.WriteEvent("", "agent_event", errEvt)
		return
	}
	readyEvt, _ := json.Marshal(map[string]interface{}{
		"type":    "snack_ready",
		"url":     snackURL,
		"message": fmt.Sprintf("Expo Snack preview ready: %s", snackURL),
	})
	_ = w.WriteEvent("", "agent_event", readyEvt)
	log.Printf("[Snack] Preview URL: %s", snackURL)
}

// ── Misc handlers ─────────────────────────────────────────────────────────────

// GetMessages handles GET /api/v1/projects/{project_id}/messages
func (h *ChatHandler) GetMessages(ctx context.Context, c *app.RequestContext) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "project_id path parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	messages, err := h.messageHistoryService.GetMessageHistory(ctx, projectID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, agentmodel.APIResponse{
			Error: &agentmodel.APIError{
				ErrorCode: "MESSAGE_HISTORY_FAILED",
				Message:   fmt.Sprintf("Failed to get message history: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	c.JSON(http.StatusOK, agentmodel.APIResponse{Data: messages})
}

// generateProjectName uses the chat model to produce a short title and persists it.
// Runs in a goroutine; errors are logged only.
func (h *ChatHandler) generateProjectName(ctx context.Context, projectID, userIDStr, firstMessage string) {
	prompt := fmt.Sprintf(
		"Generate a concise, descriptive project name (3-6 words, no quotes, no punctuation) "+
			"based on this user request: %s\n\nRespond with ONLY the project name, nothing else.",
		firstMessage,
	)
	msgs := []*schema.Message{schema.UserMessage(prompt)}
	stream, err := h.chatModel.Stream(ctx, msgs)
	if err != nil {
		log.Printf("[Naming] stream error: %v", err)
		return
	}
	var buf strings.Builder
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		if msg != nil && msg.Content != "" {
			buf.WriteString(msg.Content)
		}
	}
	name := strings.TrimSpace(buf.String())
	name = strings.Trim(name, `"'`)
	if len(name) > 60 {
		name = name[:60]
	}
	if name == "" {
		return
	}
	if err := h.projectManager.SetName(ctx, projectID, userIDStr, name); err != nil {
		log.Printf("[Naming] SetName error: %v", err)
	} else {
		log.Printf("[Naming] Project %s named: %s", projectID[:8], name)
	}
}

// generateThinkingProcess builds a short thinking description for the plan event.
func (h *ChatHandler) generateThinkingProcess(userMessage, framework string, isNewProject bool) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("User request: %s. ", userMessage))
	if isNewProject {
		b.WriteString(fmt.Sprintf("Framework: %s. ", frameworkDisplayName(framework)))
	}
	b.WriteString("Style: clean modern UI with Tailwind CSS. ")
	lower := strings.ToLower(userMessage)
	switch {
	case strings.Contains(lower, "counter"):
		b.WriteString("Features: increment, decrement, reset buttons with state display.")
	case strings.Contains(lower, "todo"):
		b.WriteString("Features: add, complete, delete items with list state management.")
	case strings.Contains(lower, "login"):
		b.WriteString("Features: username/password inputs, form validation, submit handler.")
	case strings.Contains(lower, "form"):
		b.WriteString("Features: controlled inputs, validation, submit.")
	default:
		b.WriteString("Will create appropriate components and features based on the request.")
	}
	return b.String()
}

// generatePlanSteps produces a list of plan steps for the plan event.
func (h *ChatHandler) generatePlanSteps(userMessage, framework string, isNewProject bool) []map[string]interface{} {
	steps := make([]map[string]interface{}, 0)
	id := 1

	if isNewProject {
		steps = append(steps, map[string]interface{}{
			"id":          id,
			"description": fmt.Sprintf("Initialize %s project", frameworkDisplayName(framework)),
			"executed":    false,
		})
		id++
	}

	lower := strings.ToLower(userMessage)
	switch {
	case strings.Contains(lower, "counter"):
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Create Counter component with increment/decrement/reset", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Wire Counter into App.tsx", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Add responsive styling", "executed": false},
		)
	case strings.Contains(lower, "todo"):
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Create TodoItem component", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Create TodoList with state management", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Create AddTodo input component", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Wire everything into App.tsx", "executed": false},
		)
	case strings.Contains(lower, "login"):
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Create LoginForm with username/password inputs", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Add form validation logic", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Wire LoginForm into App.tsx", "executed": false},
		)
	default:
		steps = append(steps,
			map[string]interface{}{"id": id, "description": fmt.Sprintf("Implement: %s", userMessage), "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Wire components into App.tsx with routing", "executed": false},
		)
		id++
		steps = append(steps,
			map[string]interface{}{"id": id, "description": "Add styles and interactions", "executed": false},
		)
	}

	id++
	steps = append(steps, map[string]interface{}{
		"id":          id,
		"description": "TypeScript check and build validation",
		"executed":    false,
	})
	return steps
}

// frameworkDisplayName returns the human-readable name for a framework identifier.
func frameworkDisplayName(framework string) string {
	switch framework {
	case "react":
		return "React"
	case "vue3":
		return "Vue 3"
	case "react-native":
		return "React Native (Expo)"
	default:
		return framework
	}
}

// buildSchemaMessages converts history + current message to schema.Message slice.
func buildSchemaMessages(history []agentmodel.Message, currentMessage string) []*schema.Message {
	messages := make([]*schema.Message, 0, len(history)+1)
	for _, msg := range history {
		if msg.Role == "user" {
			messages = append(messages, schema.UserMessage(msg.Content))
		} else if msg.Role == "assistant" {
			messages = append(messages, schema.AssistantMessage(msg.Content, nil))
		}
	}
	messages = append(messages, schema.UserMessage(currentMessage))
	return messages
}

// buildHistoryString formats message history as a plain-text block for the planner.
// The planner reads this via agentcontext to understand what was built in prior turns.
func buildHistoryString(history []agentmodel.Message) string {
	var sb strings.Builder
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		sb.WriteString("[")
		sb.WriteString(m.Role)
		sb.WriteString("]: ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
