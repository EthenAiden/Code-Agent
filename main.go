package main

import (
	"context"
	"log"
	"os"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/ethen-aiden/code-agent/agent/intent"
	agentmodel "github.com/ethen-aiden/code-agent/agent/model"
	"github.com/ethen-aiden/code-agent/agent/tools"
	"github.com/ethen-aiden/code-agent/config"
	"github.com/ethen-aiden/code-agent/handler"
	"github.com/ethen-aiden/code-agent/middleware"
	"github.com/ethen-aiden/code-agent/repository"
	"github.com/ethen-aiden/code-agent/service"
	"github.com/joho/godotenv"
)

// createModels initializes the planner (chat) model, executor model, and intent classifier.
func createModels(ctx context.Context) (einomodel.ToolCallingChatModel, einomodel.ToolCallingChatModel, *intent.IntentClassifier) {
	plannerModel, err := agentmodel.NewPlannerModel(ctx, nil)
	if err != nil {
		log.Fatalf("failed to create planner model: %v", err)
	}
	log.Println("✓ Planner/Chat Model initialized")

	executorModel, err := agentmodel.NewExecutorModel(ctx, nil)
	if err != nil {
		log.Fatalf("failed to create executor model: %v", err)
	}
	log.Println("✓ Executor Model initialized")

	intentClassifier := intent.NewIntentClassifier(plannerModel)
	log.Println("✓ Intent Classifier initialized")

	return plannerModel, executorModel, intentClassifier
}

func initializeTools(projectManager *service.ProjectManager, projectRoot string) []tool.BaseTool {
	allTools := make([]tool.BaseTool, 0)

	readFileTool := tools.NewReadFileTool(projectRoot)
	writeFileTool := tools.NewWriteFileTool(projectRoot)
	listDirectoryTool := tools.NewListDirectoryTool(projectRoot)
	allTools = append(allTools, readFileTool, writeFileTool, listDirectoryTool)

	scaffoldTool := tools.NewScaffoldProjectTool(projectRoot)
	allTools = append(allTools, scaffoldTool)

	typeCheckTool := tools.NewRunTypeCheckTool(projectRoot)
	buildTool := tools.NewRunBuildTool(projectRoot)
	allTools = append(allTools, typeCheckTool, buildTool)

	codeExecTool := tools.NewExecuteCodeTool(projectRoot)
	allTools = append(allTools, codeExecTool)

	projectContextTool := tools.NewGetProjectContextTool(projectManager, projectRoot)
	allTools = append(allTools, projectContextTool)

	if mcpTools := tools.LoadMCPTools(context.Background()); len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return allTools
}

// getProjectRoot returns the project root directory for file operations
func getProjectRoot() string {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
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
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	cfg := config.LoadConfig()

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

	sessionRepo := repository.NewMySQLSessionPersistenceRepository(db)
	var cacheRepo *repository.CacheRepositoryImpl
	if redisClient != nil {
		cacheRepo = repository.NewCacheRepository(redisClient, cfg.Redis.SessionTTL, cfg.Redis.EmptyTTL)
	}

	projectManager := service.NewProjectManager(sessionRepo, cacheRepo)
	messageHistoryService := service.NewMessageHistoryService(sessionRepo, cacheRepo)

	ctx := context.Background()
	log.Println("=== Initializing Agent System ===")
	chatModel, executorModel, intentClassifier := createModels(ctx)
	log.Println("=== Agent System Ready ===")

	projectRoot := getProjectRoot()
	buildHandler := handler.NewBuildHandler(projectRoot)

	sessionHandler := handler.NewProjectHandler(projectManager)
	chatHandler := handler.NewChatHandler(
		messageHistoryService,
		projectManager,
		buildHandler,
		intentClassifier,
		chatModel,
		executorModel,
		projectRoot,
	)
	fileHandler := handler.NewFileHandler(projectRoot)
	healthHandler := handler.NewHealthHandler()

	r := server.Default()

	corsMiddleware := middleware.NewCORSMiddleware()
	r.Use(corsMiddleware.Middleware())

	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Readiness)

	authMiddleware := middleware.NewAuthMiddleware()

	api := r.Group("/api/v1", authMiddleware.Middleware())
	{
		projects := api.Group("/projects")
		{
			projects.POST("", sessionHandler.CreateSession)
			projects.GET("", sessionHandler.ListSessions)
			projects.GET("/:project_id", sessionHandler.GetSession)
			projects.DELETE("/:project_id", sessionHandler.DeleteSession)
			projects.POST("/:project_id/chat", chatHandler.Chat)
			projects.GET("/:project_id/messages", chatHandler.GetMessages)
			projects.GET("/:project_id/files", fileHandler.GetFileTree)
			projects.GET("/:project_id/files/content", fileHandler.GetFileContent)
			projects.PUT("/:project_id/files/content", fileHandler.UpdateFileContent)
			projects.POST("/:project_id/build", buildHandler.BuildProject)
			projects.GET("/:project_id/preview", buildHandler.GetPreviewURL)
			projects.POST("/:project_id/stop", buildHandler.StopDevServer)
		}
	}

	port := cfg.Server.Port
	log.Printf("Server starting on port %s", port)
	r.Spin()
}
