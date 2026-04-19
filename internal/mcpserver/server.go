package mcpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/config"
	"github.com/js-beaulieu/tasks/internal/logger"
	"github.com/js-beaulieu/tasks/internal/mcpserver/tools"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func New(store *sqlite.Store, cfg config.Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "tasks",
		Version: "1.0.0",
	}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "health",
		Description: "Returns the health status of the server",
	}, healthHandler)

	if store != nil {
		mcp.AddTool(s, tools.ListProjectsTool, withLogging("list_projects", cfg, tools.ListProjectsHandler(store.Projects)))
		mcp.AddTool(s, tools.GetProjectTool, withLogging("get_project", cfg, tools.GetProjectHandler(store.Projects)))
		mcp.AddTool(s, tools.CreateProjectTool, withLogging("create_project", cfg, tools.CreateProjectHandler(store.Projects)))
		mcp.AddTool(s, tools.UpdateProjectTool, withLogging("update_project", cfg, tools.UpdateProjectHandler(store.Projects)))

		mcp.AddTool(s, tools.ListTasksTool, withLogging("list_tasks", cfg, tools.ListTasksHandler(store.Tasks)))
		mcp.AddTool(s, tools.GetTaskTool, withLogging("get_task", cfg, tools.GetTaskHandler(store.Tasks)))
		mcp.AddTool(s, tools.CreateTaskTool, withLogging("create_task", cfg, tools.CreateTaskHandler(store.Projects, store.Tasks)))
		mcp.AddTool(s, tools.UpdateTaskTool, withLogging("update_task", cfg, tools.UpdateTaskHandler(store.Projects, store.Tasks, store.Tags)))
		mcp.AddTool(s, tools.DeleteTaskTool, withLogging("delete_task", cfg, tools.DeleteTaskHandler(store.Projects, store.Tasks)))
		mcp.AddTool(s, tools.CompleteTaskTool, withLogging("complete_task", cfg, tools.CompleteTaskHandler(store.Projects, store.Tasks)))
		mcp.AddTool(s, tools.ListTagsTool, withLogging("list_tags", cfg, tools.ListTagsHandler(store.Tags)))
	}

	return s
}

func Handler(store *sqlite.Store, cfg config.Config) http.Handler {
	s := New(store, cfg)
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

func withLogging[I, O any](name string, cfg config.Config, h mcp.ToolHandlerFor[I, O]) mcp.ToolHandlerFor[I, O] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in I) (*mcp.CallToolResult, O, error) {
		log := logger.FromCtx(ctx)
		if cfg.LogDetailed {
			log.InfoContext(ctx, "→ tool call", "tool", name, "input", in)
		} else {
			log.InfoContext(ctx, "→ tool call", "tool", name)
		}
		start := time.Now()
		result, out, err := h(ctx, req, in)
		duration := time.Since(start)
		if err != nil {
			log.ErrorContext(ctx, "tool error", "tool", name, "err", err, "duration_ms", duration.Milliseconds())
		} else if cfg.LogDetailed {
			log.InfoContext(ctx, "← tool result", "tool", name, "output", out, "duration_ms", duration.Milliseconds())
		} else {
			log.InfoContext(ctx, "← tool result", "tool", name, "duration_ms", duration.Milliseconds())
		}
		return result, out, err
	}
}
