//go:build integration

package tasks_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestTasksIntegration_Update(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.HTTPProject(t, env)
	task := seed.HTTPTask(t, env, project.ID)

	res := httptestutil.Request(t, env.Handler, http.MethodPatch, "/tasks/"+task.ID, `{"name":"Updated task","status":"in_progress","position":0}`, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var updated model.Task
	httptestutil.Decode(t, res, &updated)
	if updated.Name != "Updated task" {
		t.Fatalf("Name = %q, want Updated task", updated.Name)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("Status = %q, want in_progress", updated.Status)
	}
}

func TestTasksIntegration_CreateAndListSubtasks(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.HTTPProject(t, env)
	task := seed.HTTPTask(t, env, project.ID)
	subtask := seed.HTTPSubtask(t, env, task.ID)

	res := httptestutil.Request(t, env.Handler, http.MethodGet, "/tasks/"+task.ID+"/tasks", "", env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var tasks []*model.Task
	httptestutil.Decode(t, res, &tasks)
	if !containsTask(tasks, subtask.ID) {
		t.Fatalf("subtask %q not found in subtask list", subtask.ID)
	}
}

func TestTasksIntegration_CompleteRecurringTask(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.HTTPProject(t, env)
	ctx := context.Background()

	due := "2026-05-08"
	recurrence := "FREQ=DAILY"
	recurring := &model.Task{
		ProjectID:  project.ID,
		Name:       "Daily follow-up",
		Status:     "todo",
		DueDate:    &due,
		OwnerID:    env.User.ID,
		Recurrence: &recurrence,
	}
	if err := env.Store.Tasks.Create(ctx, recurring); err != nil {
		t.Fatalf("seed recurring task: %v", err)
	}
	if err := env.Store.Tags.Add(ctx, recurring.ID, "recurring"); err != nil {
		t.Fatalf("seed recurring tag: %v", err)
	}

	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/tasks/"+recurring.ID+"/complete", `{"done_status":"done"}`, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var body struct {
		Completed *model.Task `json:"completed"`
		Next      *model.Task `json:"next"`
	}
	httptestutil.Decode(t, res, &body)
	if body.Completed == nil || body.Completed.Status != "done" {
		t.Fatalf("completed = %#v, want status done", body.Completed)
	}
	if body.Next == nil {
		t.Fatal("next = nil, want next occurrence")
	}
	if body.Next.DueDate == nil || *body.Next.DueDate != "2026-05-09" {
		t.Fatalf("next due_date = %v, want 2026-05-09", body.Next.DueDate)
	}
	if body.Next.Status != "todo" {
		t.Fatalf("next status = %q, want todo", body.Next.Status)
	}

	tags, err := env.Store.Tags.ListForTask(ctx, body.Next.ID)
	if err != nil {
		t.Fatalf("list next tags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "recurring" {
		t.Fatalf("next tags = %v, want [recurring]", tags)
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
