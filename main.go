package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/ethen-aiden/code-agent/agent/intent"
	agentmodel "github.com/ethen-aiden/code-agent/agent/model"
	"github.com/ethen-aiden/code-agent/agent/planexecute"
	"github.com/ethen-aiden/code-agent/agent/sequential"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/config"
	"github.com/ethen-aiden/code-agent/handler"
	"github.com/ethen-aiden/code-agent/middleware"
	"github.com/ethen-aiden/code-agent/repository"
	"github.com/ethen-aiden/code-agent/service"
	"github.com/joho/godotenv"
)

// createAgent creates the complete Sequential Agent with all sub-agents and tools.
// Returns the top-level agent AND the intent classifier (reused by ChatHandler for HITL).
func createAgent(ctx context.Context, projectManager *service.ProjectManager) (adk.Agent, *intent.IntentClassifier) {
	// Step 1a: Initialize Planner model (GPT) for IntentClassifier, Planner, Replanner
	plannerModel, err := agentmodel.NewPlannerModel(ctx, nil)
	if err != nil {
		log.Fatalf("failed to create planner model: %v", err)
	}
	log.Println("✓ Planner Model (GPT) initialized")

	// Step 1b: Initialize Executor model (Claude) for code generation
	executorModel, err := agentmodel.NewExecutorModel(ctx, nil)
	if err != nil {
		log.Fatalf("failed to create executor model: %v", err)
	}
	log.Println("✓ Executor Model (Claude) initialized")

	// Step 2: Initialize all tools (per-project path built at request time via context)
	projectRoot := getProjectRoot()
	allTools := initializeTools(projectManager, projectRoot)
	log.Printf("✓ Initialized %d tools", len(allTools))

	// Step 3: Initialize Intent Classifier (uses planner/GPT model)
	intentClassifier := intent.NewIntentClassifier(plannerModel)
	log.Println("✓ Intent Classifier initialized")

	// Step 4: Initialize Plan-Execute Agent with split models
	planExecuteAgent, err := planexecute.NewPlanExecuteAgent(ctx, &planexecute.PlanExecuteConfig{
		PlannerModel:  plannerModel,
		ExecutorModel: executorModel,
		Tools:         allTools,
		MaxIterations: 20,
	})
	if err != nil {
		log.Fatalf("failed to create plan-execute agent: %v", err)
	}
	log.Println("✓ Plan-Execute Agent initialized (Planner=GPT, Executor=Claude, Replanner=GPT)")

	// Step 5: Initialize Sequential Agent (top-level orchestrator)
	// Chat responses also use the planner model (lightweight GPT for conversation)
	sequentialAgent, err := sequential.NewSequentialAgent(ctx, &sequential.SequentialAgentConfig{
		IntentClassifier: intentClassifier,
		PlanExecuteAgent: planExecuteAgent,
		ChatModel:        plannerModel,
		Name:             "CodeAgent",
		Description:      "AI coding assistant with multi-agent architecture",
	})
	if err != nil {
		log.Fatalf("failed to create sequential agent: %v", err)
	}
	log.Println("✓ Sequential Agent initialized")

	return sequentialAgent, intentClassifier
}


func initializeTools(projectManager *service.ProjectManager, projectRoot string) []tool.BaseTool {
	allTools := make([]tool.BaseTool, 0)

	// File operation tools
	readFileTool := tools.NewReadFileTool(projectRoot)
	writeFileTool := tools.NewWriteFileTool(projectRoot)
	listDirectoryTool := tools.NewListDirectoryTool(projectRoot)

	allTools = append(allTools, readFileTool, writeFileTool, listDirectoryTool)

	// Framework scaffold tool (must be called first for new projects)
	scaffoldTool := tools.NewScaffoldProjectTool(projectRoot)
	allTools = append(allTools, scaffoldTool)

	// Code validation tools
	typeCheckTool := tools.NewRunTypeCheckTool(projectRoot)
	buildTool := tools.NewRunBuildTool(projectRoot)
	allTools = append(allTools, typeCheckTool, buildTool)

	// Code execution tool
	codeExecTool := tools.NewExecuteCodeTool(projectRoot)
	allTools = append(allTools, codeExecTool)

	// Project context tool
	projectContextTool := tools.NewGetProjectContextTool(projectManager, projectRoot)
	allTools = append(allTools, projectContextTool)

	// MCP tools (optional — loaded from MCP_SERVER_URL env var, e.g. Playwright MCP)
	ctx := context.Background()
	if mcpTools := tools.LoadMCPTools(ctx); len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

// getProjectRoot returns the project root directory for file operations
// Defaults to current working directory if PROJECT_ROOT env var is not set
func getProjectRoot() string {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// Default to current working directory
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to get current working directory: %v", err)
		}
		projectRoot = cwd
	}
	log.Printf("Project root: %s", projectRoot)
	return projectRoot
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database and Redis clients
	db, err := cfg.Database.InitDB()
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	redisClient, err := cfg.Redis.InitRedis()
	if err != nil {
		log.Printf("Warning: failed to initialize Redis: %v, continuing with MySQL only", err)
		redisClient = nil
	}

	// Create repository instances
	sessionRepo := repository.NewMySQLSessionPersistenceRepository(db)
	var cacheRepo *repository.CacheRepositoryImpl
	if redisClient != nil {
		cacheRepo = repository.NewCacheRepository(redisClient, cfg.Redis.SessionTTL, cfg.Redis.EmptyTTL)
	}

	// Create service instances
	projectManager := service.NewProjectManager(sessionRepo, cacheRepo)
	messageHistoryService := service.NewMessageHistoryService(sessionRepo, cacheRepo)

	// Create Sequential Agent with complete multi-agent architecture
	// This initializes: Intent Classifier, Planner, Executor, Replanner, and all tools
	ctx := context.Background()
	log.Println("=== Initializing Agent System ===")
	agent, intentClassifier := createAgent(ctx, projectManager)
	log.Println("=== Agent System Ready ===")
	log.Println()

	// Create handler instances
	sessionHandler := handler.NewProjectHandler(projectManager)
	chatHandler := handler.NewChatHandler(messageHistoryService, projectManager, agent, intentClassifier)
	healthHandler := handler.NewHealthHandler()

	// Initialize Hertz
	r := server.Default()

	// CORS middleware (must be before other routes)
	corsMiddleware := middleware.NewCORSMiddleware()
	r.Use(corsMiddleware.Middleware())

	// Health check endpoints (no authentication required)
	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Readiness)

	// Authentication middleware
	authMiddleware := middleware.NewAuthMiddleware()

	// API routes
	api := r.Group("/api/v1", authMiddleware.Middleware())
	{
		// Project management endpoints (formerly sessions)
		projects := api.Group("/projects")
		{
			// POST /api/v1/projects - Create a new project
			projects.POST("", sessionHandler.CreateSession)

			// GET /api/v1/projects - List all projects
			projects.GET("", sessionHandler.ListSessions)

			// GET /api/v1/projects/{project_id} - Get project details
			projects.GET("/:project_id", sessionHandler.GetSession)

			// DELETE /api/v1/projects/{project_id} - Delete a project
			projects.DELETE("/:project_id", sessionHandler.DeleteSession)

			// POST /api/v1/projects/{project_id}/chat - Send message to project
			projects.POST("/:project_id/chat", chatHandler.Chat)

			// GET /api/v1/projects/{project_id}/messages - Get messages for a project
			projects.GET("/:project_id/messages", chatHandler.GetMessages)
		}
	}

	// Start server
	port := cfg.Server.Port
	log.Printf("Server starting on port %s", port)
	log.Printf("Health check available at http://localhost:%s/health", port)
	log.Printf("Readiness check available at http://localhost:%s/ready", port)
	r.Spin()
}
