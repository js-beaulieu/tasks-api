package projects_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

// testUser is the authenticated user injected by AuthMiddleware in every request.
var testUser = &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}

// testProject is a project owned by testUser.
var testProject = &model.Project{
	ID:      "proj-1",
	Name:    "Alpha",
	OwnerID: "user-1",
}

// newRequest builds an authenticated request with the given body (may be nil).
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

// serve wraps the handler with AuthMiddleware and records the response.
func serve(handler http.Handler, userRepo *mock.UserRepo, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	middleware.AuthMiddleware(userRepo)(handler).ServeHTTP(w, req)
	return w
}

// defaultUserRepo returns a UserRepo mock that always succeeds for testUser.
func defaultUserRepo() *mock.UserRepo {
	return &mock.UserRepo{User: testUser}
}

// projectRepoWithAccess returns a mock wired so that Get and GetMemberRole
// always return testProject and the given role for any request.
func projectRepoWithAccess(role string) *mock.ProjectRepo {
	return &mock.ProjectRepo{
		GetFn: func(_ context.Context, _ string) (*model.Project, error) {
			return testProject, nil
		},
		GetMemberRoleFn: func(_ context.Context, _, _ string) (string, error) {
			return role, nil
		},
	}
}

// ── List projects ──────────────────────────────────────────────────────────

func TestListProjects(t *testing.T) {
	t.Run("GET /projects returns 200 and list", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			ListFn: func(_ context.Context, _ string) ([]*model.Project, error) {
				return []*model.Project{testProject}, nil
			},
		}
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodGet, "/", nil)
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got []*model.Project
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 1 || got[0].ID != testProject.ID {
			t.Errorf("unexpected body: %+v", got)
		}
	})
}

// ── Create project ─────────────────────────────────────────────────────────

func TestCreateProject(t *testing.T) {
	t.Run("POST /projects missing name returns 400", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, _ ...string) error { return nil },
		}
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodPost, "/", map[string]any{})
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

// ── ProjectCtx middleware ──────────────────────────────────────────────────

func TestProjectCtx(t *testing.T) {
	t.Run("GET /projects/{id} not found returns 404", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return nil, repo.ErrNotFound
			},
		}
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodGet, "/proj-1", nil)
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("GET /projects/{id} no access returns 403", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return testProject, nil
			},
			GetMemberRoleFn: func(_ context.Context, _, _ string) (string, error) {
				return "", repo.ErrNoAccess
			},
		}
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodGet, "/proj-1", nil)
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

// ── Role enforcement ───────────────────────────────────────────────────────

func TestRoleEnforcement(t *testing.T) {
	t.Run("PATCH /projects/{id} with read role returns 403", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodPatch, "/proj-1", map[string]any{"name": "New"})
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("DELETE /projects/{id} with modify role returns 403", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodDelete, "/proj-1", nil)
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

// ── Members ────────────────────────────────────────────────────────────────

func TestMembers(t *testing.T) {
	t.Run("POST /projects/{id}/members valid returns 201", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddMemberFn = func(_ context.Context, _ *model.ProjectMember) error { return nil }
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodPost, "/proj-1/members", map[string]any{
			"user_id": "user-2",
			"role":    model.RoleRead,
		})
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})

	t.Run("POST /projects/{id}/members invalid role returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodPost, "/proj-1/members", map[string]any{
			"user_id": "user-2",
			"role":    "superadmin",
		})
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

// ── Statuses ───────────────────────────────────────────────────────────────

func TestStatuses(t *testing.T) {
	t.Run("DELETE /projects/{id}/statuses/{status} conflict returns 409", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteStatusFn = func(_ context.Context, _, _ string) error {
			return repo.ErrConflict
		}
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodDelete, "/proj-1/statuses/todo", nil)
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", w.Code)
		}
	})

	t.Run("POST /projects/{id}/statuses empty status returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		handler := projects.NewRouter(pr, &mock.TaskRepo{})
		req := newRequest(http.MethodPost, "/proj-1/statuses", map[string]any{"status": "   "})
		w := serve(handler, defaultUserRepo(), req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
