package tasks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

var testUser = &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}

// newTestTask returns a fresh task for each test — avoids shared pointer mutations.
func newTestTask() *model.Task {
	pid := "old-parent"
	return &model.Task{
		ID:        "task-1",
		ProjectID: "proj-1",
		Name:      "Fix bug",
		Status:    "todo",
		OwnerID:   "user-1",
		ParentID:  &pid,
	}
}

func newRequest(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("X-User-ID", testUser.ID)
	req.Header.Set("X-User-Name", testUser.Name)
	req.Header.Set("X-User-Email", testUser.Email)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	userRepo := &mock.UserRepo{User: testUser}
	middleware.AuthMiddleware(userRepo)(handler).ServeHTTP(w, req)
	return w
}

// taskRepoFound returns a TaskRepo that returns a fresh task for every Get call.
func taskRepoFound() *mock.TaskRepo {
	return &mock.TaskRepo{
		GetFn: func(_ context.Context, _ string) (*model.Task, error) {
			return newTestTask(), nil
		},
	}
}

// projectRepoWithRole returns a ProjectRepo whose GetMemberRole always returns the given role.
func projectRepoWithRole(role string) *mock.ProjectRepo {
	return &mock.ProjectRepo{
		GetMemberRoleFn: func(_ context.Context, _, _ string) (string, error) {
			return role, nil
		},
	}
}

// ── GET /tasks/{id} ────────────────────────────────────────────────────────

func TestGetTask(t *testing.T) {
	t.Run("GET /task-1 returns 200", func(t *testing.T) {
		handler := tasks.NewRouter(
			projectRepoWithRole(model.RoleRead),
			taskRepoFound(),
			&mock.TagRepo{},
		)
		w := serve(handler, newRequest(http.MethodGet, "/task-1", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got model.Task
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ID != "task-1" {
			t.Errorf("id = %q, want task-1", got.ID)
		}
	})

	t.Run("GET /task-1 not found returns 404", func(t *testing.T) {
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return nil, repo.ErrNotFound
			},
		}
		handler := tasks.NewRouter(
			projectRepoWithRole(model.RoleRead),
			tr,
			&mock.TagRepo{},
		)
		w := serve(handler, newRequest(http.MethodGet, "/task-1", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}

// ── PATCH /tasks/{id} ─────────────────────────────────────────────────────

func TestPatchTask(t *testing.T) {
	t.Run("PATCH with read role returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := tasks.NewRouter(
			projectRepoWithRole(model.RoleRead),
			tr,
			&mock.TagRepo{},
		)
		w := serve(handler, newRequest(http.MethodPatch, "/task-1", map[string]any{"name": "New"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("PATCH cross-project move without target access returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error { return nil }
		// modify on proj-1 (source), no access on proj-2 (target)
		pr := &mock.ProjectRepo{
			GetMemberRoleFn: func(_ context.Context, projectID, _ string) (string, error) {
				if projectID == "proj-1" {
					return model.RoleModify, nil
				}
				return "", repo.ErrNoAccess
			},
		}
		handler := tasks.NewRouter(pr, tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPatch, "/task-1", map[string]any{
			"project_id": "proj-2",
		}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("PATCH with position change calls Update with new position", func(t *testing.T) {
		var captured *model.Task
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, t *model.Task) error {
			captured = t
			return nil
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPatch, "/task-1", map[string]any{
			"position": 3,
		}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if captured == nil {
			t.Fatal("Update was not called")
		}
		if captured.Position != 3 {
			t.Errorf("position = %d, want 3", captured.Position)
		}
	})

	t.Run(`PATCH with "parent_id": null clears parent`, func(t *testing.T) {
		var captured *model.Task
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, t *model.Task) error {
			captured = t
			return nil
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		// Send explicit null for parent_id
		body := `{"parent_id": null}`
		req := httptest.NewRequest(http.MethodPatch, "/task-1", bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", testUser.ID)
		req.Header.Set("X-User-Name", testUser.Name)
		req.Header.Set("X-User-Email", testUser.Email)
		req.Header.Set("Content-Type", "application/json")
		w := serve(handler, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if captured == nil {
			t.Fatal("Update was not called")
		}
		if captured.ParentID != nil {
			t.Errorf("ParentID = %q, want nil", *captured.ParentID)
		}
	})

	t.Run("PATCH omitting parent_id leaves parent unchanged", func(t *testing.T) {
		var captured *model.Task
		tr := taskRepoFound() // returns task with ParentID = &"old-parent"
		tr.UpdateFn = func(_ context.Context, t *model.Task) error {
			captured = t
			return nil
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPatch, "/task-1", map[string]any{
			"name": "Updated name",
		}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if captured == nil {
			t.Fatal("Update was not called")
		}
		if captured.ParentID == nil || *captured.ParentID != "old-parent" {
			t.Errorf("ParentID = %v, want &old-parent", captured.ParentID)
		}
	})

	t.Run("PATCH with invalid status returns 409", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error {
			return repo.ErrConflict
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPatch, "/task-1", map[string]any{
			"status": "nonexistent",
		}))
		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", w.Code)
		}
	})
}

// ── DELETE /tasks/{id} ────────────────────────────────────────────────────

func TestDeleteTask(t *testing.T) {
	t.Run("DELETE /task-1 returns 204", func(t *testing.T) {
		tr := taskRepoFound()
		tr.DeleteFn = func(_ context.Context, _ string) error { return nil }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodDelete, "/task-1", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})
}

// ── POST /tasks/{id}/tasks ────────────────────────────────────────────────

func TestCreateSubtask(t *testing.T) {
	t.Run("POST /task-1/tasks missing name returns 400", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CreateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPost, "/task-1/tasks", map[string]any{}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /task-1/tasks with name returns 201", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CreateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPost, "/task-1/tasks", map[string]any{
			"name": "Subtask",
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})
}

// ── POST /tasks/{id}/tags ─────────────────────────────────────────────────

func TestAddTag(t *testing.T) {
	t.Run("POST /task-1/tags returns 201", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			AddFn: func(_ context.Context, _, _ string) error { return nil },
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodPost, "/task-1/tags", map[string]any{
			"tag": "bug",
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})
}

// ── DELETE /tasks/{id}/tags/{tag} ─────────────────────────────────────────

func TestDeleteTag(t *testing.T) {
	t.Run("DELETE /task-1/tags/bug returns 204", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			DeleteTagFn: func(_ context.Context, _, _ string) error { return nil },
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodDelete, "/task-1/tags/bug", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})
}
