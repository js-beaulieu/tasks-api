//go:build integration

package tools_test

import (
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	mcptest "github.com/js-beaulieu/tasks-api/internal/testing/mcp"
)

func TestMCPProjectsIntegration_CreateListGetUpdate(t *testing.T) {
	env := mcptest.NewEnv(t)

	createResult := mcptest.CallTool(t, env, "create_project", map[string]any{
		"name":        "Test Project",
		"description": "integration project",
		"due_date":    "2026-06-01",
		"statuses":    []string{"review"},
	})
	project := mcptest.DecodeStructured[model.Project](t, createResult)

	listResult := mcptest.CallTool(t, env, "list_projects", nil)
	list := mcptest.DecodeStructured[struct {
		Projects []*model.Project `json:"projects"`
	}](t, listResult)
	if !containsProject(list.Projects, project.ID) {
		t.Fatalf("project %q not found in list_projects result", project.ID)
	}

	getResult := mcptest.CallTool(t, env, "get_project", map[string]any{"project_id": project.ID})
	got := mcptest.DecodeStructured[model.Project](t, getResult)
	if got.ID != project.ID {
		t.Fatalf("get_project ID = %q, want %q", got.ID, project.ID)
	}

	updateResult := mcptest.CallTool(t, env, "update_project", map[string]any{
		"project_id":   project.ID,
		"name":         "Updated MCP Project",
		"add_statuses": []string{"qa"},
	})
	updated := mcptest.DecodeStructured[model.Project](t, updateResult)
	if updated.Name != "Updated MCP Project" {
		t.Fatalf("updated project name = %q, want Updated MCP Project", updated.Name)
	}

	statuses, err := env.Store.Projects.ListStatuses(t.Context(), project.ID)
	if err != nil {
		t.Fatalf("list statuses: %v", err)
	}
	if !containsStatus(statuses, "review") || !containsStatus(statuses, "qa") {
		t.Fatalf("statuses = %#v, want review and qa", statuses)
	}
}

func containsProject(projects []*model.Project, id string) bool {
	for _, project := range projects {
		if project.ID == id {
			return true
		}
	}
	return false
}

func containsStatus(statuses []*model.ProjectStatus, status string) bool {
	for _, projectStatus := range statuses {
		if projectStatus.Status == status {
			return true
		}
	}
	return false
}
