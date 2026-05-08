//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/config"
	"github.com/js-beaulieu/tasks-api/internal/model"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestHTTPHappyPath(t *testing.T) {
	_, store := testdb.Open(t)
	handler := New(store, config.Config{})
	ctx := context.Background()
	user := seed.User(t, store, "u-http-1", "HTTP User", "http-user@example.com")

	t.Run("health is public", func(t *testing.T) {
		res := request(t, handler, http.MethodGet, "/health", "", "")
		assertStatus(t, res, http.StatusOK)

		var body map[string]string
		decode(t, res, &body)
		if body["status"] != "ok" {
			t.Fatalf("status = %q, want ok", body["status"])
		}
	})

	project := createProject(t, handler, user.ID)

	t.Run("list projects includes created project", func(t *testing.T) {
		res := request(t, handler, http.MethodGet, "/projects", "", user.ID)
		assertStatus(t, res, http.StatusOK)

		var projects []*model.Project
		decode(t, res, &projects)
		if !containsProject(projects, project.ID) {
			t.Fatalf("project %q not found in list", project.ID)
		}
	})

	t.Run("project statuses include defaults and custom status", func(t *testing.T) {
		res := request(t, handler, http.MethodGet, "/projects/"+project.ID+"/statuses", "", user.ID)
		assertStatus(t, res, http.StatusOK)

		var statuses []*model.ProjectStatus
		decode(t, res, &statuses)
		want := []string{"todo", "in_progress", "done", "cancelled", "review"}
		if len(statuses) != len(want) {
			t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(want))
		}
		for i, status := range statuses {
			if status.Status != want[i] {
				t.Fatalf("statuses[%d] = %q, want %q", i, status.Status, want[i])
			}
			if status.Position != i {
				t.Fatalf("statuses[%d].Position = %d, want %d", i, status.Position, i)
			}
		}
	})

	task := createTask(t, handler, user.ID, project.ID)

	t.Run("list project tasks includes created task", func(t *testing.T) {
		res := request(t, handler, http.MethodGet, "/projects/"+project.ID+"/tasks", "", user.ID)
		assertStatus(t, res, http.StatusOK)

		var tasks []*model.Task
		decode(t, res, &tasks)
		if !containsTask(tasks, task.ID) {
			t.Fatalf("task %q not found in project task list", task.ID)
		}
	})

	subtask := createSubtask(t, handler, user.ID, task.ID)

	t.Run("list subtasks includes created subtask", func(t *testing.T) {
		res := request(t, handler, http.MethodGet, "/tasks/"+task.ID+"/tasks", "", user.ID)
		assertStatus(t, res, http.StatusOK)

		var tasks []*model.Task
		decode(t, res, &tasks)
		if !containsTask(tasks, subtask.ID) {
			t.Fatalf("subtask %q not found in subtask list", subtask.ID)
		}
	})

	t.Run("add and list task tag", func(t *testing.T) {
		res := request(t, handler, http.MethodPost, "/tasks/"+task.ID+"/tags", `{"tag":"backend"}`, user.ID)
		assertStatus(t, res, http.StatusCreated)

		res = request(t, handler, http.MethodGet, "/tasks/"+task.ID+"/tags", "", user.ID)
		assertStatus(t, res, http.StatusOK)

		var tags []string
		decode(t, res, &tags)
		if len(tags) != 1 || tags[0] != "backend" {
			t.Fatalf("tags = %v, want [backend]", tags)
		}
	})

	t.Run("patch task fields", func(t *testing.T) {
		res := request(t, handler, http.MethodPatch, "/tasks/"+task.ID, `{"name":"Updated task","status":"in_progress","position":0}`, user.ID)
		assertStatus(t, res, http.StatusOK)

		var updated model.Task
		decode(t, res, &updated)
		if updated.Name != "Updated task" {
			t.Fatalf("Name = %q, want Updated task", updated.Name)
		}
		if updated.Status != "in_progress" {
			t.Fatalf("Status = %q, want in_progress", updated.Status)
		}
	})

	t.Run("complete recurring task creates next occurrence", func(t *testing.T) {
		due := "2026-05-08"
		recurrence := "FREQ=DAILY"
		recurring := &model.Task{
			ProjectID:  project.ID,
			Name:       "Daily follow-up",
			Status:     "todo",
			DueDate:    &due,
			OwnerID:    user.ID,
			Recurrence: &recurrence,
		}
		if err := store.Tasks.Create(ctx, recurring); err != nil {
			t.Fatalf("seed recurring task: %v", err)
		}
		if err := store.Tags.Add(ctx, recurring.ID, "recurring"); err != nil {
			t.Fatalf("seed recurring tag: %v", err)
		}

		res := request(t, handler, http.MethodPost, "/tasks/"+recurring.ID+"/complete", `{"done_status":"done"}`, user.ID)
		assertStatus(t, res, http.StatusOK)

		var body struct {
			Completed *model.Task `json:"completed"`
			Next      *model.Task `json:"next"`
		}
		decode(t, res, &body)
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

		tags, err := store.Tags.ListForTask(ctx, body.Next.ID)
		if err != nil {
			t.Fatalf("list next tags: %v", err)
		}
		if len(tags) != 1 || tags[0] != "recurring" {
			t.Fatalf("next tags = %v, want [recurring]", tags)
		}
	})
}

func createProject(t *testing.T, handler http.Handler, userID string) *model.Project {
	t.Helper()

	body := `{"name":"HTTP Project","description":"integration project","due_date":"2026-06-01","statuses":["review"]}`
	res := request(t, handler, http.MethodPost, "/projects", body, userID)
	assertStatus(t, res, http.StatusCreated)

	var project model.Project
	decode(t, res, &project)
	if project.ID == "" {
		t.Fatal("project ID is empty")
	}
	if project.Name != "HTTP Project" {
		t.Fatalf("project name = %q, want HTTP Project", project.Name)
	}
	return &project
}

func createTask(t *testing.T, handler http.Handler, userID, projectID string) *model.Task {
	t.Helper()

	body := `{"name":"HTTP Task","description":"integration task","status":"todo","due_date":"2026-06-02"}`
	res := request(t, handler, http.MethodPost, "/projects/"+projectID+"/tasks", body, userID)
	assertStatus(t, res, http.StatusCreated)

	var task model.Task
	decode(t, res, &task)
	if task.ID == "" {
		t.Fatal("task ID is empty")
	}
	if task.ProjectID != projectID {
		t.Fatalf("task project_id = %q, want %q", task.ProjectID, projectID)
	}
	if task.Position != 0 {
		t.Fatalf("task position = %d, want 0", task.Position)
	}
	return &task
}

func createSubtask(t *testing.T, handler http.Handler, userID, parentID string) *model.Task {
	t.Helper()

	res := request(t, handler, http.MethodPost, "/tasks/"+parentID+"/tasks", `{"name":"HTTP Subtask","status":"todo"}`, userID)
	assertStatus(t, res, http.StatusCreated)

	var task model.Task
	decode(t, res, &task)
	if task.ParentID == nil || *task.ParentID != parentID {
		t.Fatalf("subtask parent_id = %v, want %q", task.ParentID, parentID)
	}
	return &task
}

func request(t *testing.T, handler http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func assertStatus(t *testing.T, res *httptest.ResponseRecorder, want int) {
	t.Helper()

	if res.Code != want {
		t.Fatalf("status = %d, want %d, body: %s", res.Code, want, res.Body.String())
	}
}

func decode(t *testing.T, res *httptest.ResponseRecorder, v any) {
	t.Helper()

	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, res.Body.String())
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

func containsTask(tasks []*model.Task, id string) bool {
	for _, task := range tasks {
		if task.ID == id {
			return true
		}
	}
	return false
}
