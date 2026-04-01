package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/agent/intent"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/model"
	"github.com/ethen-aiden/code-agent/service"
)

// ChatHandler handles chat endpoints with session management
type ChatHandler struct {
	messageHistoryService *service.MessageHistoryService
	projectManager        tools.ProjectManagerInterface
	agent                 adk.Agent
	intentClassifier      *intent.IntentClassifier
}

// NewChatHandler creates a new ChatHandler with dependency injection
func NewChatHandler(
	messageHistoryService *service.MessageHistoryService,
	projectManager tools.ProjectManagerInterface,
	agent adk.Agent,
	intentClassifier *intent.IntentClassifier,
) *ChatHandler {
	return &ChatHandler{
		messageHistoryService: messageHistoryService,
		projectManager:        projectManager,
		agent:                 agent,
		intentClassifier:      intentClassifier,
	}
}

// clarificationPayload is the JSON body of a "clarification_needed" SSE event.
type clarificationPayload struct {
	Type    string   `json:"type"`
	Message string   `json:"message"`
	Options []string `json:"options"`
}

// Chat handles POST /api/v1/projects/{project_id}/chat
func (h *ChatHandler) Chat(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Extract project_id from path
	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "project_id path parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Parse request body
	var req model.ChatRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   fmt.Sprintf("Invalid request body: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Create session if not exists
	err := h.messageHistoryService.CreateSessionIfNotExists(ctx, projectID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "SESSION_CREATION_FAILED",
				Message:   fmt.Sprintf("Failed to create session: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Initialize ExecutionContext with project ID and session parameters
	ctx = agentcontext.InitContextParams(ctx)
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"project_id": projectID,
		"user_id":    userIDStr,
		"session_id": projectID, // Using project_id as session_id for now
	})

	// Load project context before agent invocation
	projectContext, err := h.loadProjectContext(ctx, projectID, userIDStr)
	if err != nil {
		log.Printf("Warning: Failed to load project context: %v", err)
		// Continue without project context - not a fatal error
	} else {
		// Store project context in ExecutionContext
		agentcontext.AppendContextParams(ctx, map[string]interface{}{
			"project_context": projectContext,
		})

		// ── Human-in-the-Loop: Framework Selection ──────────────────────────────
		// If the project has no framework set yet, AND the user message is a code
		// generation intent, AND the request body does NOT include a framework
		// selection → pause and ask the user to pick a framework.
		if projectContext.Framework == "" && req.Framework == "" {
			classification, classErr := h.intentClassifier.Classify(ctx, req.Message)
			if classErr == nil &&
				(classification.Intent == intent.IntentGenerateCode || classification.Intent == intent.IntentModifyCode) {
				// Persist the user message first (so conversation history is intact)
				userMsg := model.Message{
					ConversationID: projectID,
					Role:           "user",
					Content:        req.Message,
					Timestamp:      time.Now(),
				}
				_ = h.messageHistoryService.InsertMessage(ctx, userMsg, userIDStr)

				// Send the clarification_needed SSE event and stop
				w := sse.NewWriter(c)
				defer w.Close()
				payload := clarificationPayload{
					Type:    "framework_selection",
					Message: "请选择本项目使用的前端框架，以便生成对应的代码结构：",
					Options: []string{"vue3", "react", "react-native"},
				}
				payloadBytes, _ := json.Marshal(payload)
				_ = w.WriteEvent("", "clarification_needed", payloadBytes)
				_ = w.WriteEvent("", "done", []byte(""))
				return
			}
		}

		// If this request carries a framework choice (response to clarification), persist it
		if req.Framework != "" && projectContext.Framework == "" {
			if saveErr := h.projectManager.SetFramework(ctx, projectID, userIDStr, req.Framework); saveErr != nil {
				log.Printf("Warning: Failed to save framework: %v", saveErr)
			} else {
				projectContext.Framework = req.Framework
				// Update context so agents pick it up
				agentcontext.AppendContextParams(ctx, map[string]interface{}{
					"framework": req.Framework,
				})
			}
		}

		// Inject framework into agent context so Planner/Executor prompts get the constraint
		if projectContext.Framework != "" {
			agentcontext.AppendContextParams(ctx, map[string]interface{}{
				"framework": projectContext.Framework,
			})
		}
	}

	// Get message history for context
	history, err := h.messageHistoryService.GetMessageHistory(ctx, projectID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "MESSAGE_HISTORY_FAILED",
				Message:   fmt.Sprintf("Failed to get message history: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Store user message
	userMsg := model.Message{
		ConversationID: projectID,
		Role:           "user",
		Content:        req.Message,
		Timestamp:      time.Now(),
	}
	err = h.messageHistoryService.InsertMessage(ctx, userMsg, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "MESSAGE_INSERT_FAILED",
				Message:   fmt.Sprintf("Failed to store user message: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// 创建 SSE Writer
	w := sse.NewWriter(c)
	defer w.Close()

	// 检测客户端断开连接
	connClosed := ctx.Done()

	// 发送初始 ping 事件（空数据，只是为了建立连接）
	err = w.WriteEvent("", "ping", []byte(""))
	if err != nil {
		log.Printf("Failed to write ping event: %v", err)
		return
	}

	// Convert message history to Eino format
	messages := make([]adk.Message, 0, len(history)+1)
	for _, msg := range history {
		if msg.Role == "user" {
			messages = append(messages, schema.UserMessage(msg.Content))
		} else if msg.Role == "assistant" {
			messages = append(messages, schema.AssistantMessage(msg.Content, nil))
		}
	}

	// Add current user message
	messages = append(messages, schema.UserMessage(req.Message))

	// Create runner with streaming enabled using adk.RunnerConfig
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           h.agent,
		EnableStreaming: true,
	})

	// Run agent with messages
	var fullResponse strings.Builder
	iterator := runner.Run(ctx, messages)

	log.Printf("[%s] 开始运行 Agent", time.Now().Format("15:04:05.000"))

	chunkIndex := 0

	for {
		// 检查客户端是否断开
		select {
		case <-connClosed:
			log.Println("客户端断开连接，停止发送")
			return
		default:
			// 继续处理
		}

		event, ok := iterator.Next()
		if !ok {
			break
		}

		// Handle error
		if event.Err != nil {
			log.Printf("Agent error: %v", event.Err)
			w.WriteEvent("", "error", []byte(event.Err.Error()))
			return
		}

		// Handle output
		if event.Output != nil && event.Output.MessageOutput != nil {
			if event.Output.MessageOutput.IsStreaming {
				// Handle streaming output - 实时读取并发送
				stream := event.Output.MessageOutput.MessageStream
				log.Printf("开始接收流式输出...")

				for {
					// 检查客户端是否断开
					select {
					case <-connClosed:
						log.Println("客户端断开连接，停止发送")
						return
					default:
						// 继续处理
					}

					msg, err := stream.Recv()
					if err != nil {
						if err.Error() == "EOF" {
							log.Printf("[%s] 流式输出结束，共 %d 个块", time.Now().Format("15:04:05.000"), chunkIndex)
							break
						}
						log.Printf("[%s] Stream error: %v", time.Now().Format("15:04:05.000"), err)
						break
					}

					if msg != nil && msg.Content != "" {
						chunkIndex++
						fullResponse.WriteString(msg.Content)

						// 打印详细的时间戳和内容
						timestamp := time.Now().Format("15:04:05.000")
						log.Printf("[%s] 后端发送块 #%d: %q (长度: %d)", timestamp, chunkIndex, msg.Content, len(msg.Content))

						// 立即通过 SSE 发送
						eventID := fmt.Sprintf("chunk-%d", chunkIndex)
						err = w.WriteEvent(eventID, "message", []byte(msg.Content))
						if err != nil {
							log.Printf("Failed to write event: %v", err)
							return
						}

						// 小延迟确保发送
						time.Sleep(10 * time.Millisecond)
					}
				}
			} else {
				// Handle non-streaming output
				msg, err := event.Output.MessageOutput.GetMessage()
				if err == nil && msg.Content != "" {
					fullResponse.WriteString(msg.Content)
					log.Printf("收到非流式消息: %q", msg.Content)
					w.WriteEvent("", "message", []byte(msg.Content))
				}
			}
		}
	}

	// Send completion event
	log.Printf("[%s] 发送完成事件", time.Now().Format("15:04:05.000"))
	w.WriteEvent("", "done", []byte(""))

	// Store assistant message
	assistantMsg := model.Message{
		ConversationID: projectID,
		Role:           "assistant",
		Content:        fullResponse.String(),
		Timestamp:      time.Now(),
	}
	err = h.messageHistoryService.InsertMessage(ctx, assistantMsg, userIDStr)
	if err != nil {
		log.Printf("Failed to store assistant message: %v", err)
	}

	// Update message statuses to completed
	userMsgIndex, err := h.messageHistoryService.GetLastMessageIndex(ctx, projectID, userIDStr)
	if err == nil && userMsgIndex > 0 {
		h.messageHistoryService.UpdateMessageStatus(ctx, projectID, userMsgIndex, "completed")
	}

	assistantMsgIndex, err := h.messageHistoryService.GetLastMessageIndex(ctx, projectID, userIDStr)
	if err == nil && assistantMsgIndex > 0 {
		h.messageHistoryService.UpdateMessageStatus(ctx, projectID, assistantMsgIndex, "completed")
	}
}

// GetMessages handles GET /api/v1/projects/{project_id}/messages
func (h *ChatHandler) GetMessages(ctx context.Context, c *app.RequestContext) {
	// Extract user_id from context (set by auth middleware)
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_MISSING",
				Message:   "X-User-ID header is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "USER_ID_INVALID",
				Message:   "X-User-ID must be a valid UUID v4",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Extract project_id from path
	projectID := c.Param("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "INVALID_INPUT",
				Message:   "project_id path parameter is required",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Get message history
	messages, err := h.messageHistoryService.GetMessageHistory(ctx, projectID, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIResponse{
			Error: &model.APIError{
				ErrorCode: "MESSAGE_HISTORY_FAILED",
				Message:   fmt.Sprintf("Failed to get message history: %v", err),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Return messages
	c.JSON(http.StatusOK, model.APIResponse{
		Data: messages,
	})
}

// ProjectContext represents the project context information
type ProjectContext struct {
	ProjectID     string                 `json:"project_id"`
	UserID        string                 `json:"user_id"`
	Framework     string                 `json:"framework"` // "vue3", "react", "react-native", or ""
	MessageCount  int                    `json:"message_count"`
	CreatedAt     string                 `json:"created_at"`
	FileStructure map[string]interface{} `json:"file_structure,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// loadProjectContext retrieves project context including metadata and file structure
func (h *ChatHandler) loadProjectContext(ctx context.Context, projectID string, userID string) (*ProjectContext, error) {
	// Get session details from project manager
	details, err := h.projectManager.GetSession(ctx, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session details: %w", err)
	}

	// Build project context
	projectContext := &ProjectContext{
		ProjectID:    projectID,
		UserID:       userID,
		Framework:    details.Framework,
		MessageCount: details.MessageCount,
		CreatedAt:    details.CreatedAt.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"last_message_at": details.LastMessageAt,
		},
	}

	// Note: File structure retrieval would be handled by GetProjectContextTool
	// when agents need it. We don't load it here to avoid overhead.

	return projectContext, nil
}
