package tools

import (
	"context"
	"fmt"
	"log"
	"os"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// LoadMCPTools connects to a running MCP server and returns its tools wrapped as Eino tools.
// The MCP server URL is read from the environment variable MCP_SERVER_URL.
// If the variable is not set or the connection fails, it returns nil (caller should handle gracefully).
//
// Supported MCP servers:
//   - Playwright MCP (https://github.com/microsoft/playwright-mcp) for browser automation
//
// To start Playwright MCP:
//
//	npx @playwright/mcp@latest --port 8931
//
// Then set: MCP_SERVER_URL=http://localhost:8931/sse
func LoadMCPTools(ctx context.Context) []tool.BaseTool {
	serverURL := os.Getenv("MCP_SERVER_URL")
	if serverURL == "" {
		return nil
	}

	tools, err := connectMCPServer(ctx, serverURL)
	if err != nil {
		log.Printf("Warning: failed to connect to MCP server at %s: %v (browser tools unavailable)", serverURL, err)
		return nil
	}

	log.Printf("✓ Loaded %d tool(s) from MCP server at %s", len(tools), serverURL)
	return tools
}

// connectMCPServer establishes an SSE connection to the MCP server and fetches all available tools.
func connectMCPServer(ctx context.Context, serverURL string) ([]tool.BaseTool, error) {
	cli, err := client.NewSSEMCPClient(serverURL)
	if err != nil {
		return nil, fmt.Errorf("create SSE client: %w", err)
	}

	if err := cli.Start(ctx); err != nil {
		return nil, fmt.Errorf("start SSE client: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "frontend-code-agent",
		Version: "1.0.0",
	}

	if _, err := cli.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("initialize MCP session: %w", err)
	}

	tools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: cli})
	if err != nil {
		return nil, fmt.Errorf("list MCP tools: %w", err)
	}

	return tools, nil
}
