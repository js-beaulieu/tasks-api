package tasks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
	"github.com/js-beaulieu/tasks-api/internal/testing/mock"
)

var testUser = &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}

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

func rawRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", testUser.ID)
	req.Header.Set("X-User-Name", testUser.Name)
	req.Header.Set("X-User-Email", testUser.Email)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newHandler(pr *mock.ProjectRepo, tr *mock.TaskRepo, tagRepo *mock.TagRepo) http.Handler {
	return mock.NewTestRouter(&mock.UserRepo{User: testUser}, func(api huma.API) {
		tasks.Register(api, pr, tr, tagRepo)
	})
}

func taskRepoFound() *mock.TaskRepo {
	return &mock.TaskRepo{
		GetFn: func(_ context.Context, _ string) (*model.Task, error) {
			return newTestTask(), nil
		},
	}
}

func projectRepoWithRole(role string) *mock.ProjectRepo {
	return &mock.ProjectRepo{
		GetFn: func(_ context.Context, _ string) (*model.Project, error) {
			return &model.Project{ID: "proj-1", OwnerID: "user-1"}, nil
		},
		GetMemberRoleFn: func(_ context.Context, _, _ string) (string, error) {
			return role, nil
		},
	}
}

func TestGetTask(t *testing.T) {
	t.Run("GET /tasks/{id} returns 200", func(t *testing.T) {
		handler := newHandler(projectRepoWithRole(model.RoleRead), taskRepoFound(), &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1", nil))
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

	t.Run("GET /tasks/{id} not found returns 404", func(t *testing.T) {
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return nil, repo.ErrNotFound
			},
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}

func TestPatchTask(t *testing.T) {
	t.Run("PATCH /tasks/{id} with read role returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPatch, "/tasks/task-1", map[string]any{"name": "New"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("PATCH /tasks/{id} cross-project move without target access returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error { return nil }
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return &model.Project{ID: "proj-1", OwnerID: "user-1"}, nil
			},
			GetMemberRoleFn: func(_ context.Context, projectID, _ string) (string, error) {
				if projectID == "proj-1" {
					return model.RoleModify, nil
				}
				return "", repo.ErrNoAccess
			},
		}
		handler := newHandler(pr, tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPatch, "/tasks/task-1", map[string]any{
			"project_id": "proj-2",
		}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("PATCH /tasks/{id} with position change calls Update with new position", func(t *testing.T) {
		var captured *model.Task
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, t *model.Task) error {
			captured = t
			return nil
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPatch, "/tasks/task-1", map[string]any{
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

	t.Run(`PATCH /tasks/{id} with "parent_id": null clears parent`, func(t *testing.T) {
		var captured *model.Task
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, t *model.Task) error {
			captured = t
			return nil
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		req := httptest.NewRequest(http.MethodPatch, "/tasks/task-1", bytes.NewBufferString(`{"parent_id": null}`))
		req.Header.Set("X-User-ID", testUser.ID)
		req.Header.Set("X-User-Name", testUser.Name)
		req.Header.Set("X-User-Email", testUser.Email)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
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

	t.Run("PATCH /tasks/{id} omitting parent_id leaves parent unchanged", func(t *testing.T) {
		var captured *model.Task
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, t *model.Task) error {
			captured = t
			return nil
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPatch, "/tasks/task-1", map[string]any{
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

	t.Run("PATCH /tasks/{id} with invalid status returns 409", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error {
			return repo.ErrConflict
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPatch, "/tasks/task-1", map[string]any{
			"status": "nonexistent",
		}))
		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", w.Code)
		}
	})
}

func TestDeleteTask(t *testing.T) {
	t.Run("DELETE /tasks/{id} returns 204", func(t *testing.T) {
		tr := taskRepoFound()
		tr.DeleteFn = func(_ context.Context, _ string) error { return nil }
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodDelete, "/tasks/task-1", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})
}

func TestCreateSubtask(t *testing.T) {
	t.Run("POST /tasks/{id}/tasks missing name returns 400", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CreateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/tasks", map[string]any{}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /tasks/{id}/tasks with name returns 201", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CreateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/tasks", map[string]any{
			"name": "Subtask",
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})
}

func TestAddTag(t *testing.T) {
	t.Run("POST /tasks/{id}/tags returns 201", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			AddFn: func(_ context.Context, _, _ string) error { return nil },
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/tags", map[string]any{
			"tag": "bug",
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})
}

func TestCompleteTask(t *testing.T) {
	t.Run("POST /tasks/{id}/complete non-recurring returns 200 with next=null", func(t *testing.T) {
		completed := newTestTask()
		completed.Status = "done"

		tr := taskRepoFound()
		tr.CompleteTaskFn = func(_ context.Context, _, _ string) (*model.Task, *model.Task, error) {
			return completed, nil, nil
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/complete", map[string]any{
			"done_status": "done",
		}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp struct {
			Completed *model.Task `json:"completed"`
			Next      *model.Task `json:"next"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Completed == nil {
			t.Error("completed is nil")
		}
		if resp.Next != nil {
			t.Errorf("next = %v, want null for non-recurring", resp.Next)
		}
	})

	t.Run("POST /tasks/{id}/complete recurring returns 200 with next task", func(t *testing.T) {
		completed := newTestTask()
		completed.Status = "done"
		nextDue := "2026-04-15"
		nextTask := &model.Task{
			ID:        "task-2",
			ProjectID: "proj-1",
			Name:      "Fix bug",
			Status:    "todo",
			DueDate:   &nextDue,
		}

		tr := taskRepoFound()
		tr.CompleteTaskFn = func(_ context.Context, _, _ string) (*model.Task, *model.Task, error) {
			return completed, nextTask, nil
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/complete", map[string]any{
			"done_status": "done",
		}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var resp struct {
			Completed *model.Task `json:"completed"`
			Next      *model.Task `json:"next"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Completed == nil {
			t.Error("completed is nil")
		}
		if resp.Next == nil {
			t.Fatal("next is nil, want next occurrence")
		}
		if resp.Next.ID != "task-2" {
			t.Errorf("next.ID = %q, want task-2", resp.Next.ID)
		}
	})

	t.Run("POST /tasks/{id}/complete with read role returns 403", func(t *testing.T) {
		handler := newHandler(projectRepoWithRole(model.RoleRead), taskRepoFound(), &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/complete", map[string]any{
			"done_status": "done",
		}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("POST /tasks/{id}/complete invalid done_status returns 409", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CompleteTaskFn = func(_ context.Context, _, _ string) (*model.Task, *model.Task, error) {
			return nil, nil, repo.ErrConflict
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/complete", map[string]any{
			"done_status": "nonexistent",
		}))
		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", w.Code)
		}
	})
}

func TestDeleteTag(t *testing.T) {
	t.Run("DELETE /tasks/{id}/tags/{tag} returns 204", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			DeleteTagFn: func(_ context.Context, _, _ string) error { return nil },
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodDelete, "/tasks/task-1/tags/bug", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})
}

func TestTaskCtxInternalError(t *testing.T) {
	t.Run("GET /tasks/{id} task Get returns non-NotFound error gives 500", func(t *testing.T) {
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return nil, errors.New("db error")
			},
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestPatchTaskExtra(t *testing.T) {
	t.Run("PATCH /tasks/{id} invalid JSON returns 400", func(t *testing.T) {
		handler := newHandler(projectRepoWithRole(model.RoleModify), taskRepoFound(), &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, rawRequest(http.MethodPatch, "/tasks/task-1", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /tasks/{id} repo internal error returns 500", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error { return errors.New("db error") }
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPatch, "/tasks/task-1", map[string]any{"name": "X"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteTaskExtra(t *testing.T) {
	t.Run("DELETE /tasks/{id} read role returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.DeleteFn = func(_ context.Context, _ string) error { return nil }
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodDelete, "/tasks/task-1", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("DELETE /tasks/{id} repo error returns 500", func(t *testing.T) {
		tr := taskRepoFound()
		tr.DeleteFn = func(_ context.Context, _ string) error { return errors.New("db error") }
		handler := newHandler(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodDelete, "/tasks/task-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestListSubtasks(t *testing.T) {
	t.Run("GET /tasks/{id}/tasks success returns 200", func(t *testing.T) {
		tr := taskRepoFound()
		tr.ListChildrenFn = func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
			return []*model.Task{}, nil
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1/tasks", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /tasks/{id}/tasks repo error returns 500", func(t *testing.T) {
		tr := taskRepoFound()
		tr.ListChildrenFn = func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
			return nil, errors.New("db error")
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1/tasks", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestCreateSubtaskExtra(t *testing.T) {
	t.Run("POST /tasks/{id}/tasks read role returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CreateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := newHandler(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/tasks", map[string]any{"name": "Sub"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestListTags(t *testing.T) {
	t.Run("GET /tasks/{id}/tags success returns 200", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListForTaskFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{"bug", "urgent"}, nil
			},
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1/tags", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /tasks/{id}/tags nil list returns empty array", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListForTaskFn: func(_ context.Context, _ string) ([]string, error) {
				return nil, nil
			},
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1/tags", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /tasks/{id}/tags repo error returns 500", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListForTaskFn: func(_ context.Context, _ string) ([]string, error) {
				return nil, errors.New("db error")
			},
		}
		handler := newHandler(projectRepoWithRole(model.RoleRead), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodGet, "/tasks/task-1/tags", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestAddTagExtra(t *testing.T) {
	t.Run("POST /tasks/{id}/tags whitespace-only tag returns 400", func(t *testing.T) {
		handler := newHandler(projectRepoWithRole(model.RoleModify), taskRepoFound(), &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/tags", map[string]any{"tag": "   "}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /tasks/{id}/tags repo error returns 500", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			AddFn: func(_ context.Context, _, _ string) error { return errors.New("db error") },
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodPost, "/tasks/task-1/tags", map[string]any{"tag": "bug"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteTagExtra(t *testing.T) {
	t.Run("DELETE /tasks/{id}/tags/{tag} read role returns 403", func(t *testing.T) {
		handler := newHandler(projectRepoWithRole(model.RoleRead), taskRepoFound(), &mock.TagRepo{})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodDelete, "/tasks/task-1/tags/bug", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("DELETE /tasks/{id}/tags/{tag} repo error returns 500", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			DeleteTagFn: func(_ context.Context, _, _ string) error { return errors.New("db error") },
		}
		handler := newHandler(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newRequest(http.MethodDelete, "/tasks/task-1/tags/bug", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}
