//go:build integration

package tools_test

import (
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	mcptest "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/mcp"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/seed"
)

func TestMCPTasksIntegration_CreateListGetUpdateAndComplete(t *testing.T) {
	env := mcptest.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})

	createResult := mcptest.CallTool(t, env, "create_task", map[string]any{
		"project_id":  project.ID,
		"name":        "Test Task",
		"description": "integration task",
		"status":      "todo",
		"due_date":    "2026-06-02",
	})
	task := mcptest.Decode[model.Task](t, createResult)

	listResult := mcptest.CallTool(t, env, "list_tasks", map[string]any{"project_id": project.ID})
	list := mcptest.Decode[struct {
		Tasks []*model.Task `json:"tasks"`
	}](t, listResult)
	if !containsTask(list.Tasks, task.ID) {
		t.Fatalf("task %q not found in list_tasks result", task.ID)
	}

	getResult := mcptest.CallTool(t, env, "get_task", map[string]any{"task_id": task.ID})
	got := mcptest.Decode[model.Task](t, getResult)
	if got.ID != task.ID {
		t.Fatalf("get_task ID = %q, want %q", got.ID, task.ID)
	}

	updateResult := mcptest.CallTool(t, env, "update_task", map[string]any{
		"task_id":  task.ID,
		"name":     "Updated MCP Task",
		"status":   "in_progress",
		"add_tags": []string{"backend"},
	})
	updated := mcptest.Decode[struct {
		*model.Task
		Tags             []string `json:"tags"`
		NextOccurrenceID *string  `json:"next_occurrence_id,omitempty"`
	}](t, updateResult)
	if updated.Task == nil {
		t.Fatal("updated task is nil")
	}
	if updated.Name != "Updated MCP Task" {
		t.Fatalf("updated task name = %q, want Updated MCP Task", updated.Name)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("updated task status = %q, want in_progress", updated.Status)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "backend" {
		t.Fatalf("updated tags = %v, want [backend]", updated.Tags)
	}

	due := "2026-05-08"
	recurrence := "FREQ=DAILY"
	recurring := &model.Task{
		ProjectID:  project.ID,
		Name:       "Daily MCP Task",
		Status:     "todo",
		DueDate:    &due,
		OwnerID:    env.User.ID,
		Recurrence: &recurrence,
	}
	if err := env.Store.Tasks.Create(t.Context(), recurring); err != nil {
		t.Fatalf("seed recurring task: %v", err)
	}

	completeResult := mcptest.CallTool(t, env, "update_task", map[string]any{
		"task_id": recurring.ID,
		"status":  "done",
	})
	completed := mcptest.Decode[struct {
		*model.Task
		Tags             []string `json:"tags"`
		NextOccurrenceID *string  `json:"next_occurrence_id,omitempty"`
	}](t, completeResult)
	if completed.Task == nil || completed.Status != "done" {
		t.Fatalf("completed = %#v, want status done", completed.Task)
	}
	if completed.NextOccurrenceID == nil {
		t.Fatal("next_occurrence_id = nil, want next occurrence ID")
	}

	// Verify the spawned task exists and has the expected due date
	nextTask, err := env.Store.Tasks.Get(t.Context(), *completed.NextOccurrenceID)
	if err != nil {
		t.Fatalf("get next occurrence: %v", err)
	}
	if nextTask.DueDate == nil || *nextTask.DueDate != "2026-05-09" {
		t.Fatalf("next due_date = %v, want 2026-05-09", nextTask.DueDate)
	}
}

func TestMCPTasksIntegration_CreateAndListSubtasks(t *testing.T) {
	env := mcptest.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	createResult := mcptest.CallTool(t, env, "create_task", map[string]any{
		"project_id": task.ProjectID,
		"parent_id":  task.ID,
		"name":       "Test Task",
		"status":     "todo",
	})
	subtask := mcptest.Decode[model.Task](t, createResult)

	listResult := mcptest.CallTool(t, env, "list_tasks", map[string]any{"parent_id": task.ID})
	list := mcptest.Decode[struct {
		Tasks []*model.Task `json:"tasks"`
	}](t, listResult)
	if !containsTask(list.Tasks, subtask.ID) {
		t.Fatalf("subtask %q not found in list_tasks result", subtask.ID)
	}
}

func containsTask(tasks []*model.Task, id string) bool {
	for _, task := range tasks {
		if task.ID == id {
			return true
		}
	}
	return false
}
