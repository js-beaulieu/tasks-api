//go:build integration

package tools_test

import (
	"testing"

	mcptest "github.com/js-beaulieu/tasks-api/internal/testing/mcp"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestMCPTagsIntegration_ListTags(t *testing.T) {
	env := mcptest.NewEnv(t)
	project := seed.Project(t, env.Store, env.User.ID)
	task := seed.Task(t, env.Store, project.ID, env.User.ID, nil)

	mcptest.CallTool(t, env, "update_task", map[string]any{
		"task_id":  task.ID,
		"add_tags": []string{"backend"},
	})

	result := mcptest.CallTool(t, env, "list_tags", nil)
	list := mcptest.DecodeStructured[struct {
		Tags []string `json:"tags"`
	}](t, result)
	if len(list.Tags) != 1 || list.Tags[0] != "backend" {
		t.Fatalf("tags = %v, want [backend]", list.Tags)
	}
}
