package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

// ── Event-type constants ──────────────────────────────────────────────────────
//
// These mirror the SSE vocabulary the frontend already understands.
// Using typed constants (Claude Code pattern) instead of bare strings throughout.

const (
	sseTypePlan            = "plan"
	sseTypePlanStepStart   = "plan_step_start"
	sseTypePlanStepDone    = "plan_step_done"
	sseTypeToolCall        = "tool_call"
	sseTypeRepairFile      = "repair_file"
	sseTypeValidationPass  = "validation_pass"
	sseTypeValidationError = "validation_error"
	sseTypeStatus          = "status"
)

// ── Agent-name constants ──────────────────────────────────────────────────────
//
// The Eino ADK sets these from the Name() method of each agent in the pipeline.
// Confirmed from planexecute.NewPlanner / NewExecutor / NewReplanner constructors.

const (
	agentNamePlanner   = "planner"
	agentNameExecutor  = "executor"
	agentNameReplanner = "replanner"
)

// ── Tool-name constants ───────────────────────────────────────────────────────

const (
	toolWriteFile = "write_file"
	toolScaffold  = "scaffold_project"
	toolTypeCheck = "run_type_check"
	toolBuild     = "run_build"
	toolReadFile  = "read_file"
	toolListDir   = "list_directory"
)

// bridgeResult is returned after draining the AgentEvent stream.
type bridgeResult struct {
	summary      string
	writtenPaths []string
	planSteps    []map[string]interface{}
	hasError     bool
	errMsg       string
}

// pendingToolCall carries a tool call's structured arguments from the assistant
// message, so we can use them when the corresponding tool result arrives.
// (Claude Code pattern: read args from ToolCalls[], not from result strings.)
type pendingToolCall struct {
	name string
	args map[string]interface{}
}

// consumeAgentEvents drains an AsyncIterator[*AgentEvent] and translates each
// event into the existing SSE vocabulary expected by the frontend.
//
// Architecture follows Claude Code's three-layer switch pattern:
//
//	Layer 1: evt.Err / evt.Output / evt.Action  (AgentEvent discriminator)
//	Layer 2: msgVariant.Role                    (Tool vs Assistant)
//	Layer 3: agentName / toolName               (which agent/tool produced it)
func consumeAgentEvents(
	iter *adk.AsyncIterator[*adk.AgentEvent],
	tw *traceWriter,
	w *sse.Writer,
) bridgeResult {
	res := bridgeResult{}

	stepIndex := 0
	inRepair := false
	repairRound := 0
	planEmitted := false

	// pending maps ToolCallID → structured args from the preceding assistant msg.
	// Populated when Role==Assistant with ToolCalls, consumed when Role==Tool.
	pending := make(map[string]pendingToolCall)

	for {
		evt, ok := iter.Next()
		if !ok {
			// channel closed — normal end of stream.
			break
		}
		if evt == nil {
			continue
		}

		// ── Layer 1: AgentEvent discriminator ────────────────────────────────

		// 1a. Error
		if evt.Err != nil {
			log.Printf("[SSEBridge] agent error: %+v", evt.Err)
			// Unwrap to surface the root cause
			cause := evt.Err
			for {
				unwrapped := errors.Unwrap(cause)
				if unwrapped == nil {
					break
				}
				cause = unwrapped
			}
			log.Printf("[SSEBridge] root cause: %+v", cause)
			res.hasError = true
			res.errMsg = evt.Err.Error()
			_ = w.WriteEvent("", "error", []byte(evt.Err.Error()))
			continue
		}

		// 1b. Action events (BreakLoop = replanner finished)
		if evt.Action != nil {
			if evt.Action.BreakLoop != nil {
				// Pipeline complete — emit final step_done
				doneEvt := mustMarshal(map[string]interface{}{
					"type":       sseTypePlanStepDone,
					"step_index": stepIndex,
				})
				tw.emit(doneEvt)
			}
			continue
		}

		// 1c. Session KV dump (CustomizedOutput from outputSessionKVsAgent)
		if evt.Output != nil && evt.Output.CustomizedOutput != nil {
			res.summary = extractSummaryFromKVs(evt.Output.CustomizedOutput)
			continue
		}

		// 1d. Must have MessageOutput to proceed
		if evt.Output == nil || evt.Output.MessageOutput == nil {
			continue
		}

		mv := evt.Output.MessageOutput
		agentName := agentNameFromPath(evt.RunPath)

		// ── Layer 2: Role discriminator ───────────────────────────────────────

		switch mv.Role {

		case schema.Tool:
			// Tool result: look up structured args we saved from the assistant msg.
			handleToolResult(mv, pending, tw, &res, &inRepair, &repairRound)

		case schema.Assistant:
			msg := drainMessage(mv)
			if msg == nil {
				continue
			}

			// Save pending tool calls BEFORE dispatching by agent name,
			// so they're available when the tool result arrives.
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				if args == nil {
					args = make(map[string]interface{})
				}
				pending[tc.ID] = pendingToolCall{name: tc.Function.Name, args: args}
			}

			// ── Layer 3: Agent-name discriminator ────────────────────────────
			switch agentName {

			case agentNamePlanner:
				if planEmitted {
					break
				}
				planEmitted = true
				dispatchPlannerMessage(msg, tw, &res, &stepIndex)

			case agentNameExecutor:
				// Emit a status for each tool the executor is about to call.
				for _, tc := range msg.ToolCalls {
					toolLabel := tc.Function.Name
					// For write_file, append the path so users can see what file is being written.
					if toolLabel == toolWriteFile {
						var targs map[string]interface{}
						if json.Unmarshal([]byte(tc.Function.Arguments), &targs) == nil {
							if p, ok := targs["path"].(string); ok && p != "" {
								toolLabel = "write_file: " + p
							}
						}
					}
					tw.emit(mustMarshal(map[string]interface{}{
						"type":    sseTypeStatus,
						"message": fmt.Sprintf("Calling %s…", toolLabel),
					}))
				}
				// Surface plain-text executor replies (no tool call) as a status + log.
				if len(msg.ToolCalls) == 0 && msg.Content != "" {
					log.Printf("[Executor] no-tool-call text: %s", truncate(msg.Content, 300))
					tw.emit(mustMarshal(map[string]interface{}{
						"type":    sseTypeStatus,
						"message": truncate(msg.Content, 120),
					}))
				}

			case agentNameReplanner:
				// Log replanner decision so it's visible in server logs.
				if msg.Content != "" {
					log.Printf("[Replanner] decision: %s", truncate(msg.Content, 300))
				}
				for _, tc := range msg.ToolCalls {
					log.Printf("[Replanner] tool=%s args=%s", tc.Function.Name, truncate(tc.Function.Arguments, 200))
				}
				// Replanner fired — close current step, open next.
				doneEvt := mustMarshal(map[string]interface{}{
					"type":       sseTypePlanStepDone,
					"step_index": stepIndex,
				})
				tw.emit(doneEvt)
				stepIndex++
				startEvt := mustMarshal(map[string]interface{}{
					"type":       sseTypePlanStepStart,
					"step_index": stepIndex,
				})
				tw.emit(startEvt)
			}
		}
	}

	return res
}

// dispatchPlannerMessage translates the planner's assistant message into a
// plan SSE event. The planner returns a JSON plan via tool call arguments.
func dispatchPlannerMessage(
	msg *schema.Message,
	tw *traceWriter,
	res *bridgeResult,
	stepIndex *int,
) {
	// Planner uses create_plan tool — get steps from ToolCalls if present.
	var plan []map[string]interface{}

	for _, tc := range msg.ToolCalls {
		if tc.Function.Name != "create_plan" {
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			continue
		}
		plan = stepsFromPlanArgs(args)
		break
	}

	// Fallback: planner might have put JSON in Content (non-tool mode).
	if plan == nil {
		plan = parsePlanFromContent(msg.Content)
	}

	if plan != nil {
		res.planSteps = plan
		planEvt := mustMarshal(map[string]interface{}{
			"type": sseTypePlan,
			"content": map[string]interface{}{
				"goal":  goalFromPlanArgs(msg),
				"steps": plan,
			},
		})
		tw.emit(planEvt)
		startEvt := mustMarshal(map[string]interface{}{
			"type":       sseTypePlanStepStart,
			"step_index": *stepIndex,
		})
		tw.emit(startEvt)
	} else {
		// Planner returned plain text — surface as status.
		statusEvt := mustMarshal(map[string]interface{}{
			"type":    sseTypeStatus,
			"message": truncate(msg.Content, 120),
		})
		tw.emit(statusEvt)
	}
}

// handleToolResult translates a tool-result event into SSE frames.
// Uses structured args from the preceding assistant message (pending map)
// rather than parsing the result string — the Claude Code pattern.
func handleToolResult(
	mv *adk.MessageVariant,
	pending map[string]pendingToolCall,
	tw *traceWriter,
	res *bridgeResult,
	inRepair *bool,
	repairRound *int,
) {
	toolName := mv.ToolName
	// Resolve result content — tool results are never streaming.
	// adk.Message is a type alias for *schema.Message — use directly.
	var content string
	if mv.Message != nil {
		content = mv.Message.Content
	}

	// Look up the structured args from the matching pending entry.
	// ToolCallID is on the tool-result message.
	var ptc pendingToolCall
	if mv.Message != nil && mv.Message.ToolCallID != "" {
		if p, found := pending[mv.Message.ToolCallID]; found {
			ptc = p
			delete(pending, mv.Message.ToolCallID)
		}
	}
	// Fall back: find first pending entry with matching name.
	if ptc.name == "" {
		for id, p := range pending {
			if p.name == toolName {
				ptc = p
				delete(pending, id)
				break
			}
		}
	}

	// ── Layer 3: Tool-name discriminator ─────────────────────────────────────
	switch toolName {

	case toolWriteFile:
		// Path comes from structured args — reliable, no string parsing needed.
		path, _ := ptc.args["path"].(string)
		if path == "" {
			path = extractPathFromResult(content) // fallback
		}
		if path != "" {
			res.writtenPaths = append(res.writtenPaths, path)
		}
		evtType := sseTypeToolCall
		if *inRepair {
			evtType = sseTypeRepairFile
		}
		evt := mustMarshal(map[string]interface{}{
			"type":    evtType,
			"tool":    toolWriteFile,
			"path":    path,
			"message": fmt.Sprintf("Written: %s", path),
		})
		tw.emit(evt)

	case toolScaffold:
		evt := mustMarshal(map[string]interface{}{
			"type":    sseTypeToolCall,
			"tool":    toolScaffold,
			"message": "Project scaffolded",
		})
		tw.emit(evt)

	case toolTypeCheck, toolBuild:
		var vr struct {
			Success bool   `json:"success"`
			Stdout  string `json:"stdout"`
			Stderr  string `json:"stderr"`
		}
		if err := json.Unmarshal([]byte(content), &vr); err == nil {
			if vr.Success {
				*inRepair = false
				*repairRound = 0
				evt := mustMarshal(map[string]interface{}{
					"type":    sseTypeValidationPass,
					"message": fmt.Sprintf("%s passed", toolName),
				})
				tw.emit(evt)
			} else {
				*inRepair = true
				*repairRound++
				combined := strings.TrimSpace(vr.Stdout + "\n" + vr.Stderr)
				evt := mustMarshal(map[string]interface{}{
					"type":    sseTypeValidationError,
					"round":   *repairRound,
					"message": truncate(combined, 600),
				})
				tw.emit(evt)
			}
		}

	case toolReadFile, toolListDir:
		// Silently consumed — no SSE needed.

	default:
		evt := mustMarshal(map[string]interface{}{
			"type":    sseTypeStatus,
			"message": fmt.Sprintf("%s: %s", toolName, truncate(content, 100)),
		})
		tw.emit(evt)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// agentNameFromPath returns the leaf agent name from an event's RunPath.
func agentNameFromPath(path []adk.RunStep) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1].String()
}

// drainMessage fully resolves a MessageVariant into a *schema.Message.
// For streaming variants it drains all chunks first (Claude Code Layer 1 pattern:
// stream is fully consumed before the business layer sees it).
func drainMessage(mv *adk.MessageVariant) *schema.Message {
	if !mv.IsStreaming {
		// adk.Message is a type alias for *schema.Message — no assertion needed.
		return mv.Message
	}
	if mv.MessageStream == nil {
		return nil
	}
	var buf strings.Builder
	var toolCalls []schema.ToolCall
	for {
		chunk, err := mv.MessageStream.Recv()
		if err != nil {
			break
		}
		if chunk == nil {
			continue
		}
		buf.WriteString(chunk.Content)
		toolCalls = append(toolCalls, chunk.ToolCalls...)
	}
	return &schema.Message{
		Role:      schema.Assistant,
		Content:   buf.String(),
		ToolCalls: toolCalls,
	}
}

// stepsFromPlanArgs converts create_plan tool args → step slice.
func stepsFromPlanArgs(args map[string]interface{}) []map[string]interface{} {
	stepsRaw, ok := args["steps"]
	if !ok {
		return nil
	}
	stepsSlice, ok := stepsRaw.([]interface{})
	if !ok {
		return nil
	}
	steps := make([]map[string]interface{}, 0, len(stepsSlice))
	for i, s := range stepsSlice {
		switch v := s.(type) {
		case map[string]interface{}:
			if _, hasID := v["id"]; !hasID {
				v["id"] = i + 1
			}
			if _, hasDone := v["executed"]; !hasDone {
				v["executed"] = false
			}
			steps = append(steps, v)
		case string:
			steps = append(steps, map[string]interface{}{
				"id":          i + 1,
				"description": v,
				"executed":    false,
			})
		}
	}
	return steps
}

// parsePlanFromContent is the fallback when the planner emits JSON in Content
// rather than through a tool call.
func parsePlanFromContent(content string) []map[string]interface{} {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end <= start {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content[start:end+1]), &raw); err != nil {
		return nil
	}
	return stepsFromPlanArgs(raw)
}

// goalFromPlanArgs extracts the goal string from the planner's create_plan tool call.
func goalFromPlanArgs(msg *schema.Message) string {
	for _, tc := range msg.ToolCalls {
		if tc.Function.Name != "create_plan" {
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			continue
		}
		if g, ok := args["goal"].(string); ok && g != "" {
			return g
		}
	}
	return "Build project"
}

// extractSummaryFromKVs reads the final session KV dump.
func extractSummaryFromKVs(kvs interface{}) string {
	m, ok := kvs.(map[string]interface{})
	if !ok {
		return ""
	}
	if raw, ok := m["ExecutedSteps"]; ok {
		if steps, ok := raw.([]interface{}); ok && len(steps) > 0 {
			return fmt.Sprintf("Completed %d steps", len(steps))
		}
	}
	return ""
}

// extractPathFromResult is the fallback path extractor from a tool result string.
// Only used when structured args are unavailable.
func extractPathFromResult(content string) string {
	content = strings.TrimSpace(content)
	// write_file returns "success: file written to <path>"
	if idx := strings.LastIndex(content, " "); idx != -1 {
		last := content[idx+1:]
		if strings.Contains(last, "/") || strings.Contains(last, ".") {
			return last
		}
	}
	return content
}

// mustMarshal marshals v to JSON, returning nil on error (safe for tw.emit).
func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// truncate shortens s to at most n bytes, appending "…" if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
