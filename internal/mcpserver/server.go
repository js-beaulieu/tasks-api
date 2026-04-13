package mcpserver

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func New() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "tasks",
		Version: "1.0.0",
	}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "health",
		Description: "Returns the health status of the server",
	}, healthHandler)
	return s
}

func Handler() http.Handler {
	s := New()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s
	}, nil)
}

func healthHandler(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: `{"status":"ok"}`},
		},
	}, nil, nil
}
