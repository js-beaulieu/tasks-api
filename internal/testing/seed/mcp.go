package seed

import (
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	mcptest "github.com/js-beaulieu/tasks-api/internal/testing/mcp"
)

func MCPProject(t *testing.T, env *mcptest.Env) *model.Project {
	t.Helper()

	result := mcptest.CallTool(t, env, "create_project", map[string]any{
		"name":        "MCP Project",
		"description": "integration project",
		"due_date":    "2026-06-01",
		"statuses":    []string{"review"},
	})
	project := mcptest.DecodeStructured[model.Project](t, result)
	return &project
}

func MCPTask(t *testing.T, env *mcptest.Env, projectID string) *model.Task {
	t.Helper()

	result := mcptest.CallTool(t, env, "create_task", map[string]any{
		"project_id":  projectID,
		"name":        "MCP Task",
		"description": "integration task",
		"status":      "todo",
		"due_date":    "2026-06-02",
	})
	task := mcptest.DecodeStructured[model.Task](t, result)
	return &task
}

func MCPSubtask(t *testing.T, env *mcptest.Env, parent *model.Task) *model.Task {
	t.Helper()

	result := mcptest.CallTool(t, env, "create_task", map[string]any{
		"project_id": parent.ProjectID,
		"parent_id":  parent.ID,
		"name":       "MCP Subtask",
		"status":     "todo",
	})
	task := mcptest.DecodeStructured[model.Task](t, result)
	return &task
}
