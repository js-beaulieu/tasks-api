package mcpserver

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/mcpserver/tools"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func New(store *sqlite.Store) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "tasks",
		Version: "1.0.0",
	}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "health",
		Description: "Returns the health status of the server",
	}, healthHandler)

	if store != nil {
		mcp.AddTool(s, tools.ListProjectsTool, tools.ListProjectsHandler(store.Projects))
		mcp.AddTool(s, tools.GetProjectTool, tools.GetProjectHandler(store.Projects))
		mcp.AddTool(s, tools.CreateProjectTool, tools.CreateProjectHandler(store.Projects))
		mcp.AddTool(s, tools.UpdateProjectTool, tools.UpdateProjectHandler(store.Projects))

		mcp.AddTool(s, tools.ListTasksTool, tools.ListTasksHandler(store.Tasks))
		mcp.AddTool(s, tools.GetTaskTool, tools.GetTaskHandler(store.Tasks))
		mcp.AddTool(s, tools.CreateTaskTool, tools.CreateTaskHandler(store.Tasks))
		mcp.AddTool(s, tools.UpdateTaskTool, tools.UpdateTaskHandler(store.Tasks))
		mcp.AddTool(s, tools.DeleteTaskTool, tools.DeleteTaskHandler(store.Projects, store.Tasks))
		mcp.AddTool(s, tools.ListTagsTool, tools.ListTagsHandler(store.Tags))
	}

	return s
}

func Handler(store *sqlite.Store) http.Handler {
	s := New(store)
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
