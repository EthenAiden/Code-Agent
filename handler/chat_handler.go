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
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/agent/intent"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/agent/validation"
	agentmodel "github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/service"
)

// ChatHandler handles chat endpoints with session management
type ChatHandler struct {
	messageHistoryService *service.MessageHistoryService
	projectManager        tools.ProjectManagerInterface
	buildHandler          *BuildHandler
	intentClassifier      *intent.IntentClassifier
	chatModel             einomodel.ToolCallingChatModel
	executorModel         einomodel.ToolCallingChatModel
	projectRoot           string
}

// NewChatHandler creates a new ChatHandler with dependency injection
func NewChatHandler(
	messageHistoryService *service.MessageHistoryService,
	projectManager tools.ProjectManagerInterface,
	buildHandler *BuildHandler,
	intentClassifier *intent.IntentClassifier,
	chatModel einomodel.ToolCallingChatModel,
	executorModel einomodel.ToolCallingChatModel,
	projectRoot string,
) *ChatHandler {
	return &ChatHandler{
		messageHistoryService: messageHistoryService,
		projectManager:        projectManager,
		buildHandler:          buildHandler,
		intentClassifier:      intentClassifier,
		chatModel:             chatModel,
		executorModel:         executorModel,
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

	return fmt.Sprintf(`You are an expert frontend code generator — think Lovable, Bolt, or v0.
Your job is to take a user request and produce COMPLETE, WORKING code files immediately.

## Output Format — MANDATORY

You MUST output every file you create or modify inside a <file> tag:

<file path="src/App.tsx">
[complete file content — no truncation, no TODOs]
</file>

<file path="src/components/Counter.tsx">
[complete file content]
</file>

After all <file> blocks, write a short plain-text summary (1-3 lines) explaining what you built.

## Rules

1. ONLY output <file> blocks + a brief summary. No long explanations, no markdown outside the blocks.
2. Every file must be COMPLETE — all imports, exports, types, and logic included.
3. Use Tailwind CSS utility classes for ALL styling. The project already has tailwindcss configured with src/index.css importing @tailwind directives. ALWAYS use Tailwind classes — never write raw CSS files or style tags unless absolutely necessary.
4. The app must run immediately after writing — no placeholder TODOs, no "implement this later".
5. Keep components in separate files only when they are reused. Simple apps can live in App.tsx.
6. Always include src/App.tsx (the root component).
7. NEVER output plain HTML files. ALWAYS use the React framework.

%s
%s`, frameworkConstraints, existingSection)
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
		// Skip node_modules and dist
		if strings.HasPrefix(rel, "node_modules/") || strings.HasPrefix(rel, "dist/") {
			return nil
		}
		lines = append(lines, "- "+rel)
		return nil
	})
	return strings.Join(lines, "\n")
}

// Chat handles POST /api/v1/projects/{project_id}/chat
func (h *ChatHandler) Chat(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
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

	// Create session if not exists
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

	// Initialize ExecutionContext
	ctx = agentcontext.InitContextParams(ctx)
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": projectID,
		"user_id":    userIDStr,
		"session_id": projectID,
	})

	// Persist user message
	userMsg := agentmodel.Message{
		ConversationID: projectID,
		Role:           "user",
		Content:        req.Message,
		Timestamp:      time.Now(),
	}
	if err := h.messageHistoryService.InsertMessage(ctx, userMsg, userIDStr); err != nil {
		log.Printf("Warning: failed to store user message: %v", err)
	}

	// Get message history
	history, err := h.messageHistoryService.GetMessageHistory(ctx, projectID, userIDStr)
	if err != nil {
		log.Printf("Warning: failed to get message history: %v", err)
	}

	// Classify intent
	classification, classErr := h.intentClassifier.Classify(ctx, req.Message)

	// ── SSE setup ────────────────────────────────────────────────────────────
	w := sse.NewWriter(c)
	defer w.Close()

	// Send initial ping
	if err := w.WriteEvent("", "ping", []byte("")); err != nil {
		log.Printf("Failed to write ping: %v", err)
		return
	}

	// ── Chat intent: direct conversational reply ──────────────────────────────
	if classErr != nil || classification.Intent == intent.IntentChat {
		h.handleChatReply(ctx, c, w, projectID, userIDStr, req.Message, history)
		return
	}

	// ── Code generation / modification intent: Lovable-style generation ───────
	h.handleCodeGeneration(ctx, c, w, projectID, userIDStr, req.Message, history, req)
}

// handleChatReply sends a streaming conversational response via the chat model
func (h *ChatHandler) handleChatReply(
	ctx context.Context,
	c *app.RequestContext,
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

	// Persist assistant reply
	assistantMsg := agentmodel.Message{
		ConversationID: projectID,
		Role:           "assistant",
		Content:        fullResponse.String(),
		Timestamp:      time.Now(),
	}
	_ = h.messageHistoryService.InsertMessage(ctx, assistantMsg, userIDStr)
}

// handleCodeGeneration runs the full closed-loop generation pipeline:
// 1. Determine framework (from request or auto-detect)
// 2. Auto-scaffold if new project
// 3. Call executor model with framework-aware system prompt
// 4. Parse <file> blocks → write to disk
// 5. Validation loop: TSC → Build (≤3 self-repair rounds)
// 6. Auto-trigger dev server
func (h *ChatHandler) handleCodeGeneration(
	ctx context.Context,
	c *app.RequestContext,
	w *sse.Writer,
	projectID, userIDStr, userMessage string,
	history []agentmodel.Message,
	req agentmodel.ChatRequest,
) {
	isNewProject := !h.projectHasFiles(projectID)

	// ── Step 1: Determine framework ──────────────────────────────────────────
	var framework string
	if isNewProject {
		// Use framework from request, default to "react"
		framework = req.Framework
		if framework == "" {
			framework = "react"
		}
	} else {
		framework = h.detectFramework(projectID)
	}

	// ── Step 2: Auto-scaffold if new project ─────────────────────────────────
	if isNewProject {
		statusEvent, _ := json.Marshal(map[string]interface{}{
			"type":    "status",
			"message": fmt.Sprintf("🏗️ 初始化 %s 项目结构...", framework),
		})
		_ = w.WriteEvent("", "agent_event", statusEvent)

		if err := h.autoScaffold(projectID, framework); err != nil {
			log.Printf("Auto-scaffold error: %v", err)
			_ = w.WriteEvent("", "error", []byte("项目初始化失败: "+err.Error()))
			return
		}

		// Persist framework in project manager
		_ = h.projectManager.SetFramework(ctx, projectID, userIDStr, framework)

		scaffoldEvent, _ := json.Marshal(map[string]interface{}{
			"type":    "tool_call",
			"tool":    "scaffold_project",
			"message": fmt.Sprintf("✅ %s 项目已初始化", framework),
		})
		_ = w.WriteEvent("", "agent_event", scaffoldEvent)
	}

	// ── Step 3: Collect existing file listing ────────────────────────────────
	existingFiles := h.listExistingFiles(projectID)
	systemPrompt := buildSystemPrompt(framework, existingFiles)

	// ── Step 4: Build conversation for the executor model ───────────────────
	buildMessages := func(extraUserMessage string) []*schema.Message {
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
		msgs = append(msgs, schema.UserMessage(userMessage))
		if extraUserMessage != "" {
			msgs = append(msgs, schema.UserMessage(extraUserMessage))
		}
		return msgs
	}

	// callExecutor calls the LLM and accumulates the full XML output
	callExecutor := func(extraUserMessage string) (string, error) {
		stream, err := h.executorModel.Stream(ctx, buildMessages(extraUserMessage))
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

	// writeFiles parses <file> blocks from output and writes them to disk.
	// Returns the list of written paths and the summary text.
	writeFiles := func(output string) ([]string, string) {
		generatedFiles := parseFileBlocks(output)
		var writtenPaths []string
		for _, f := range generatedFiles {
			if err := h.writeProjectFile(projectID, f.Path, f.Content); err != nil {
				log.Printf("[CodeGen] Failed to write %s: %v", f.Path, err)
				continue
			}
			writtenPaths = append(writtenPaths, f.Path)
			toolEvent, _ := json.Marshal(map[string]interface{}{
				"type":    "tool_call",
				"tool":    "write_file",
				"args":    fmt.Sprintf(`{"path": "%s"}`, f.Path),
				"message": fmt.Sprintf("📄 写入文件: %s", f.Path),
			})
			_ = w.WriteEvent("", "agent_event", toolEvent)
			log.Printf("[CodeGen] Written: %s", f.Path)
		}
		summary := strings.TrimSpace(fileBlockRe.ReplaceAllString(output, ""))
		if summary == "" && len(writtenPaths) > 0 {
			summary = fmt.Sprintf("✅ 已生成 %d 个文件：%s", len(writtenPaths), strings.Join(writtenPaths, ", "))
		}
		return writtenPaths, summary
	}

	genEvent, _ := json.Marshal(map[string]interface{}{
		"type":    "status",
		"message": "⚡ 正在生成代码...",
	})
	_ = w.WriteEvent("", "agent_event", genEvent)

	// ── Step 5: First generation attempt ─────────────────────────────────────
	output, err := callExecutor("")
	if err != nil {
		log.Printf("Executor model error: %v", err)
		_ = w.WriteEvent("", "error", []byte("代码生成失败: "+err.Error()))
		return
	}
	log.Printf("[CodeGen] Raw output length: %d chars", len(output))

	if len(parseFileBlocks(output)) == 0 {
		// Model didn't use XML format — stream as plain message
		log.Printf("[CodeGen] No <file> blocks found, streaming as message")
		_ = w.WriteEvent("", "message", []byte(output))
		_ = w.WriteEvent("", "done", []byte(""))
		_ = h.messageHistoryService.InsertMessage(ctx, agentmodel.Message{
			ConversationID: projectID, Role: "assistant", Content: output, Timestamp: time.Now(),
		}, userIDStr)
		return
	}

	writtenPaths, summary := writeFiles(output)

	// ── Step 6: Validation + Self-repair loop (≤3 rounds) ────────────────────
	absProjectDir := filepath.Join(h.projectRoot, projectID)

	const maxRepairRounds = 3
	for round := 0; round < maxRepairRounds; round++ {
		// TSC check
		valStartEvent, _ := json.Marshal(map[string]interface{}{
			"type":    "status",
			"message": fmt.Sprintf("🔍 TypeScript 检查中（第 %d 轮）...", round+1),
		})
		_ = w.WriteEvent("", "agent_event", valStartEvent)

		tscResult := validation.RunTSC(absProjectDir, framework)
		log.Printf("[Validation] TSC round %d: passed=%v", round+1, tscResult.Passed)

		var failedResults []*validation.Result
		if !tscResult.Passed {
			failedResults = append(failedResults, tscResult)
		} else {
			// TSC passed — run build check
			buildCheckEvent, _ := json.Marshal(map[string]interface{}{
				"type":    "status",
				"message": fmt.Sprintf("🏗️ 构建检查中（第 %d 轮）...", round+1),
			})
			_ = w.WriteEvent("", "agent_event", buildCheckEvent)

			buildResult := validation.RunBuild(absProjectDir, framework)
			log.Printf("[Validation] Build round %d: passed=%v", round+1, buildResult.Passed)

			if !buildResult.Passed {
				failedResults = append(failedResults, buildResult)
			}
		}

		if len(failedResults) == 0 {
			// TSC + Build passed — run E2E (only on React/Vue web projects with a dev server)
			if framework != "react-native" {
				devPort := 5173 // default vite port; actual port is tracked in buildHandler but not exposed here
				e2eURL := fmt.Sprintf("http://host.docker.internal:%d", devPort)
				e2eCheckEvent, _ := json.Marshal(map[string]interface{}{
					"type":    "status",
					"message": "🧪 E2E 冒烟测试中...",
				})
				_ = w.WriteEvent("", "agent_event", e2eCheckEvent)

				e2eResult := validation.RunE2E(e2eURL)
				log.Printf("[Validation] E2E round %d: passed=%v", round+1, e2eResult.Passed)

				if !e2eResult.Passed && round < maxRepairRounds-1 {
					failedResults = append(failedResults, e2eResult)
				}
				// If E2E fails on last round or passes, we still break — E2E failure is advisory
			}

			if len(failedResults) == 0 {
				// All checks passed
				passEvent, _ := json.Marshal(map[string]interface{}{
					"type":    "validation_pass",
					"message": "✅ 代码校验通过",
				})
				_ = w.WriteEvent("", "agent_event", passEvent)
				break
			}
		}

		// Stream validation errors to the frontend
		errSummary := validation.FormatErrorsForLLM(failedResults)
		valErrEvent, _ := json.Marshal(map[string]interface{}{
			"type":    "validation_error",
			"round":   round + 1,
			"message": errSummary,
		})
		_ = w.WriteEvent("", "agent_event", valErrEvent)

		if round == maxRepairRounds-1 {
			// Last round — give up
			log.Printf("[Validation] Max repair rounds reached, giving up")
			break
		}

		// Self-repair: ask model to fix errors
		repairEvent, _ := json.Marshal(map[string]interface{}{
			"type":    "status",
			"message": fmt.Sprintf("🔧 自动修复中（第 %d/%d 轮）...", round+1, maxRepairRounds),
		})
		_ = w.WriteEvent("", "agent_event", repairEvent)

		repairPrompt := fmt.Sprintf(
			"The code you just generated has validation errors. Please fix ALL of them and output ONLY the corrected files using <file> blocks.\n\nErrors:\n%s",
			errSummary,
		)

		repairOutput, repairErr := callExecutor(repairPrompt)
		if repairErr != nil {
			log.Printf("[Repair] Round %d error: %v", round+1, repairErr)
			break
		}

		newPaths, newSummary := writeFiles(repairOutput)
		if len(newPaths) > 0 {
			writtenPaths = newPaths
			summary = newSummary
		}
	}

	// ── Step 7: Stream summary text ───────────────────────────────────────────
	if summary == "" {
		summary = fmt.Sprintf("✅ 已生成 %d 个文件：%s", len(writtenPaths), strings.Join(writtenPaths, ", "))
	}
	_ = w.WriteEvent("", "message", []byte(summary))

	// ── Step 8: For React Native, push to Expo Snack for cloud preview ────────
	if framework == "react-native" {
		go func() {
			snackEvent, _ := json.Marshal(map[string]interface{}{
				"type":    "status",
				"message": "📱 正在推送到 Expo Snack...",
			})
			_ = w.WriteEvent("", "agent_event", snackEvent)

			snackURL, err := tools.PushToSnack(absProjectDir, projectID)
			if err != nil {
				log.Printf("[Snack] Push failed: %v", err)
				errEvent, _ := json.Marshal(map[string]interface{}{
					"type":    "status",
					"message": "⚠️ Expo Snack 推送失败，使用本地 Expo Web 预览",
				})
				_ = w.WriteEvent("", "agent_event", errEvent)
			} else {
				snackReadyEvent, _ := json.Marshal(map[string]interface{}{
					"type":    "snack_ready",
					"url":     snackURL,
					"message": fmt.Sprintf("📱 Expo Snack 预览已就绪: %s", snackURL),
				})
				_ = w.WriteEvent("", "agent_event", snackReadyEvent)
				log.Printf("[Snack] Preview URL: %s", snackURL)
			}
		}()
	}

	// ── Step 9: Auto-trigger dev server ──────────────────────────────────────
	go func() {
		time.Sleep(500 * time.Millisecond)
		projectDir := filepath.Join(h.projectRoot, projectID)
		h.buildHandler.StartDevServer(projectID, projectDir, userIDStr, framework)
	}()

	buildEvent, _ := json.Marshal(map[string]interface{}{
		"type":    "status",
		"message": "🚀 正在启动开发服务器...",
	})
	_ = w.WriteEvent("", "agent_event", buildEvent)
	_ = w.WriteEvent("", "done", []byte(""))

	// Persist assistant message
	_ = h.messageHistoryService.InsertMessage(ctx, agentmodel.Message{
		ConversationID: projectID, Role: "assistant", Content: summary, Timestamp: time.Now(),
	}, userIDStr)
}

// buildSchemaMessages converts history + current message to schema.Message slice
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

	c.JSON(http.StatusOK, agentmodel.APIResponse{
		Data: messages,
	})
}
