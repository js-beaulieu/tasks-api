package projects_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

// rawRequest builds an authenticated request with a raw string body (for invalid-JSON tests).
func rawRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", testUser.ID)
	req.Header.Set("X-User-Name", testUser.Name)
	req.Header.Set("X-User-Email", testUser.Email)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestListProjectsRepoError(t *testing.T) {
	t.Run("GET / repo error returns 500", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			ListFn: func(_ context.Context, _ string) ([]*model.Project, error) {
				return nil, errors.New("db error")
			},
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreateProjectExtra(t *testing.T) {
	t.Run("POST / invalid JSON returns 400", func(t *testing.T) {
		w := serve(projects.NewRouter(&mock.ProjectRepo{}, &mock.TaskRepo{}), defaultUserRepo(), rawRequest(http.MethodPost, "/", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST / repo error returns 500", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, _ ...string) error {
				return errors.New("db error")
			},
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/", map[string]any{"name": "P"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── ProjectCtx ────────────────────────────────────────────────────────────────

func TestProjectCtxInternalError(t *testing.T) {
	t.Run("GET /{id} get returns non-NotFound error gives 500", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return nil, errors.New("db error")
			},
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUpdateProject(t *testing.T) {
	t.Run("PATCH /{id} success updates fields and returns 200", func(t *testing.T) {
		newName := "Beta"
		pr := projectRepoWithAccess(model.RoleModify)
		pr.UpdateFn = func(_ context.Context, _ *model.Project) error { return nil }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPatch, "/proj-1", map[string]any{"name": &newName}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("PATCH /{id} invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), rawRequest(http.MethodPatch, "/proj-1", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /{id} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		pr.UpdateFn = func(_ context.Context, _ *model.Project) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPatch, "/proj-1", map[string]any{"name": "X"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDeleteProject(t *testing.T) {
	t.Run("DELETE /{id} success returns 204", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteFn = func(_ context.Context, _ string) error { return nil }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})

	t.Run("DELETE /{id} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteFn = func(_ context.Context, _ string) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Members ───────────────────────────────────────────────────────────────────

func TestListMembers(t *testing.T) {
	t.Run("GET /{id}/members success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListMembersFn = func(_ context.Context, _ string) ([]*model.ProjectMember, error) {
			return []*model.ProjectMember{{ProjectID: "proj-1", UserID: "user-2", Role: model.RoleRead}}, nil
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1/members", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /{id}/members repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListMembersFn = func(_ context.Context, _ string) ([]*model.ProjectMember, error) {
			return nil, errors.New("db error")
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1/members", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestAddMemberExtra(t *testing.T) {
	t.Run("POST /{id}/members missing user_id returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/members", map[string]any{"user_id": "   ", "role": model.RoleRead}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /{id}/members cannot add yourself returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		// testUser.ID == "user-1" — adding yourself
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/members", map[string]any{"user_id": "user-1", "role": model.RoleRead}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /{id}/members repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddMemberFn = func(_ context.Context, _ *model.ProjectMember) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/members", map[string]any{"user_id": "user-2", "role": model.RoleRead}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestUpdateMember(t *testing.T) {
	t.Run("PATCH /{id}/members/{userID} success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.UpdateMemberRoleFn = func(_ context.Context, _, _, _ string) error { return nil }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPatch, "/proj-1/members/user-2", map[string]any{"role": model.RoleModify}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("PATCH /{id}/members/{userID} invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), rawRequest(http.MethodPatch, "/proj-1/members/user-2", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /{id}/members/{userID} invalid role returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPatch, "/proj-1/members/user-2", map[string]any{"role": "owner"}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /{id}/members/{userID} changing owner role returns 400", func(t *testing.T) {
		// testProject.OwnerID == "user-1"
		pr := projectRepoWithAccess(model.RoleAdmin)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPatch, "/proj-1/members/user-1", map[string]any{"role": model.RoleRead}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /{id}/members/{userID} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.UpdateMemberRoleFn = func(_ context.Context, _, _, _ string) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPatch, "/proj-1/members/user-2", map[string]any{"role": model.RoleRead}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestRemoveMember(t *testing.T) {
	t.Run("DELETE /{id}/members/{userID} success returns 204", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.RemoveMemberFn = func(_ context.Context, _, _ string) error { return nil }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1/members/user-2", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})

	t.Run("DELETE /{id}/members/{userID} removing owner returns 400", func(t *testing.T) {
		// testProject.OwnerID == "user-1"
		pr := projectRepoWithAccess(model.RoleAdmin)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1/members/user-1", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("DELETE /{id}/members/{userID} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.RemoveMemberFn = func(_ context.Context, _, _ string) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1/members/user-2", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Statuses ──────────────────────────────────────────────────────────────────

func TestListStatuses(t *testing.T) {
	t.Run("GET /{id}/statuses success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListStatusesFn = func(_ context.Context, _ string) ([]*model.ProjectStatus, error) {
			return []*model.ProjectStatus{{Status: "todo", Position: 0}}, nil
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1/statuses", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /{id}/statuses repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListStatusesFn = func(_ context.Context, _ string) ([]*model.ProjectStatus, error) {
			return nil, errors.New("db error")
		}
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1/statuses", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestAddStatus(t *testing.T) {
	t.Run("POST /{id}/statuses success returns 201", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddStatusFn = func(_ context.Context, _, _ string) error { return nil }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/statuses", map[string]any{"status": "review"}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})

	t.Run("POST /{id}/statuses invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), rawRequest(http.MethodPost, "/proj-1/statuses", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /{id}/statuses repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddStatusFn = func(_ context.Context, _, _ string) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/statuses", map[string]any{"status": "review"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteStatusExtra(t *testing.T) {
	t.Run("DELETE /{id}/statuses/{status} success returns 204", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteStatusFn = func(_ context.Context, _, _ string) error { return nil }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1/statuses/done", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})

	t.Run("DELETE /{id}/statuses/{status} internal error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteStatusFn = func(_ context.Context, _, _ string) error { return errors.New("db error") }
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodDelete, "/proj-1/statuses/done", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

// ── Tasks ─────────────────────────────────────────────────────────────────────

func TestListTasks(t *testing.T) {
	t.Run("GET /{id}/tasks success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		tr := &mock.TaskRepo{
			ListChildrenFn: func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
				return []*model.Task{}, nil
			},
		}
		w := serve(projects.NewRouter(pr, tr), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1/tasks", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /{id}/tasks repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		tr := &mock.TaskRepo{
			ListChildrenFn: func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
				return nil, errors.New("db error")
			},
		}
		w := serve(projects.NewRouter(pr, tr), defaultUserRepo(), newRequest(http.MethodGet, "/proj-1/tasks", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestCreateTaskExtra(t *testing.T) {
	t.Run("POST /{id}/tasks with read role returns 403", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/tasks", map[string]any{"name": "T"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("POST /{id}/tasks invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		w := serve(projects.NewRouter(pr, &mock.TaskRepo{}), defaultUserRepo(), rawRequest(http.MethodPost, "/proj-1/tasks", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /{id}/tasks success returns 201", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		tr := &mock.TaskRepo{
			CreateFn: func(_ context.Context, _ *model.Task) error { return nil },
		}
		w := serve(projects.NewRouter(pr, tr), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/tasks", map[string]any{"name": "T"}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})

	t.Run("POST /{id}/tasks repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		tr := &mock.TaskRepo{
			CreateFn: func(_ context.Context, _ *model.Task) error { return errors.New("db error") },
		}
		w := serve(projects.NewRouter(pr, tr), defaultUserRepo(), newRequest(http.MethodPost, "/proj-1/tasks", map[string]any{"name": "T"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}
