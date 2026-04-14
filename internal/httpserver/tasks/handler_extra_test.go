package tasks_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

// rawRequest builds an authenticated request with a raw string body.
func rawRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", testUser.ID)
	req.Header.Set("X-User-Name", testUser.Name)
	req.Header.Set("X-User-Email", testUser.Email)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ── taskCtx ───────────────────────────────────────────────────────────────────

func TestTaskCtxInternalError(t *testing.T) {
	t.Run("GET /{id} task Get returns non-NotFound error gives 500", func(t *testing.T) {
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return nil, errors.New("db error")
			},
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodGet, "/task-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestPatchTaskExtra(t *testing.T) {
	t.Run("PATCH /{id} invalid JSON returns 400", func(t *testing.T) {
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), taskRepoFound(), &mock.TagRepo{})
		w := serve(handler, rawRequest(http.MethodPatch, "/task-1", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /{id} repo internal error returns 500", func(t *testing.T) {
		tr := taskRepoFound()
		tr.UpdateFn = func(_ context.Context, _ *model.Task) error { return errors.New("db error") }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPatch, "/task-1", map[string]any{"name": "X"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDeleteTaskExtra(t *testing.T) {
	t.Run("DELETE /{id} read role returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.DeleteFn = func(_ context.Context, _ string) error { return nil }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodDelete, "/task-1", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("DELETE /{id} repo error returns 500", func(t *testing.T) {
		tr := taskRepoFound()
		tr.DeleteFn = func(_ context.Context, _ string) error { return errors.New("db error") }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodDelete, "/task-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Subtasks ──────────────────────────────────────────────────────────────────

func TestListSubtasks(t *testing.T) {
	t.Run("GET /{id}/tasks success returns 200", func(t *testing.T) {
		tr := taskRepoFound()
		tr.ListChildrenFn = func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
			return []*model.Task{}, nil
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodGet, "/task-1/tasks", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /{id}/tasks repo error returns 500", func(t *testing.T) {
		tr := taskRepoFound()
		tr.ListChildrenFn = func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
			return nil, errors.New("db error")
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodGet, "/task-1/tasks", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestCreateSubtaskExtra(t *testing.T) {
	t.Run("POST /{id}/tasks read role returns 403", func(t *testing.T) {
		tr := taskRepoFound()
		tr.CreateFn = func(_ context.Context, _ *model.Task) error { return nil }
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), tr, &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPost, "/task-1/tasks", map[string]any{"name": "Sub"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

// ── Tags ──────────────────────────────────────────────────────────────────────

func TestListTags(t *testing.T) {
	t.Run("GET /{id}/tags success returns 200", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListForTaskFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{"bug", "urgent"}, nil
			},
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodGet, "/task-1/tags", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /{id}/tags nil list returns empty array", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListForTaskFn: func(_ context.Context, _ string) ([]string, error) {
				return nil, nil
			},
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodGet, "/task-1/tags", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /{id}/tags repo error returns 500", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListForTaskFn: func(_ context.Context, _ string) ([]string, error) {
				return nil, errors.New("db error")
			},
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodGet, "/task-1/tags", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestAddTagExtra(t *testing.T) {
	t.Run("POST /{id}/tags whitespace-only tag returns 400", func(t *testing.T) {
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), taskRepoFound(), &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodPost, "/task-1/tags", map[string]any{"tag": "   "}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /{id}/tags repo error returns 500", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			AddFn: func(_ context.Context, _, _ string) error { return errors.New("db error") },
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodPost, "/task-1/tags", map[string]any{"tag": "bug"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteTagExtra(t *testing.T) {
	t.Run("DELETE /{id}/tags/{tag} read role returns 403", func(t *testing.T) {
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleRead), taskRepoFound(), &mock.TagRepo{})
		w := serve(handler, newRequest(http.MethodDelete, "/task-1/tags/bug", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("DELETE /{id}/tags/{tag} repo error returns 500", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			DeleteTagFn: func(_ context.Context, _, _ string) error { return errors.New("db error") },
		}
		handler := tasks.NewRouter(projectRepoWithRole(model.RoleModify), taskRepoFound(), tagRepo)
		w := serve(handler, newRequest(http.MethodDelete, "/task-1/tags/bug", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}
