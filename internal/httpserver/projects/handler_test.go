package projects_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
	"github.com/js-beaulieu/tasks-api/internal/testing/mock"
)

var testUser = &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
var testProject = &model.Project{ID: "proj-1", Name: "Alpha", OwnerID: "user-1"}

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

func newHandler(pr *mock.ProjectRepo, tr *mock.TaskRepo) http.Handler {
	return mock.NewTestRouter(&mock.UserRepo{User: testUser}, func(api huma.API) {
		projects.Register(api, pr, tr)
	})
}

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

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestListProjects(t *testing.T) {
	t.Run("GET /projects/ returns 200 and list", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			ListFn: func(_ context.Context, _ string) ([]*model.Project, error) {
				return []*model.Project{testProject}, nil
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects", nil))
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

func TestCreateProject(t *testing.T) {
	t.Run("POST /projects/ missing name returns 400", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, _ ...string) error { return nil },
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects", map[string]any{}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /projects/ with statuses forwards them to Create", func(t *testing.T) {
		var gotStatuses []string
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, additionalStatuses ...string) error {
				gotStatuses = additionalStatuses
				return nil
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects", map[string]any{
			"name":     "Ma liste",
			"statuses": []string{"À faire", "En cours", "En attente"},
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
		want := []string{"À faire", "En cours", "En attente"}
		if len(gotStatuses) != len(want) {
			t.Fatalf("gotStatuses = %v, want %v", gotStatuses, want)
		}
		for i, s := range want {
			if gotStatuses[i] != s {
				t.Errorf("gotStatuses[%d] = %q, want %q", i, gotStatuses[i], s)
			}
		}
	})

	t.Run("POST /projects/ without statuses passes empty variadic", func(t *testing.T) {
		var gotStatuses []string
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, additionalStatuses ...string) error {
				gotStatuses = additionalStatuses
				return nil
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects", map[string]any{"name": "Simple"}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
		if len(gotStatuses) != 0 {
			t.Errorf("gotStatuses = %v, want empty", gotStatuses)
		}
	})
}

func TestProjectCtx(t *testing.T) {
	t.Run("GET /projects/{id} not found returns 404", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return nil, repo.ErrNotFound
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1", nil))
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
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestRoleEnforcement(t *testing.T) {
	t.Run("PATCH /projects/{id} with read role returns 403", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1", map[string]any{"name": "New"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("DELETE /projects/{id} with modify role returns 403", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestMembers(t *testing.T) {
	t.Run("POST /projects/{id}/members valid returns 201", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddMemberFn = func(_ context.Context, _ *model.ProjectMember) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/members", map[string]any{
			"user_id": "user-2",
			"role":    model.RoleRead,
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})

	t.Run("POST /projects/{id}/members invalid role returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/members", map[string]any{
			"user_id": "user-2",
			"role":    "superadmin",
		}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

func TestStatuses(t *testing.T) {
	t.Run("DELETE /projects/{id}/statuses/{status} conflict returns 409", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteStatusFn = func(_ context.Context, _, _ string) error {
			return repo.ErrConflict
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1/statuses/todo", nil))
		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", w.Code)
		}
	})

	t.Run("POST /projects/{id}/statuses empty status returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/statuses", map[string]any{"status": "   "}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

func TestListProjectsRepoError(t *testing.T) {
	t.Run("GET /projects/ repo error returns 500", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			ListFn: func(_ context.Context, _ string) ([]*model.Project, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestCreateProjectExtra(t *testing.T) {
	t.Run("POST /projects/ invalid JSON returns 400", func(t *testing.T) {
		h := newHandler(&mock.ProjectRepo{}, &mock.TaskRepo{})
		w := serve(h, rawRequest(http.MethodPost, "/projects", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /projects/ repo error returns 500", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, _ ...string) error {
				return errors.New("db error")
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects", map[string]any{"name": "P"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestProjectCtxInternalError(t *testing.T) {
	t.Run("GET /projects/{id} get returns non-NotFound error gives 500", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestUpdateProject(t *testing.T) {
	t.Run("PATCH /projects/{id} success updates fields and returns 200", func(t *testing.T) {
		newName := "Beta"
		pr := projectRepoWithAccess(model.RoleModify)
		pr.UpdateFn = func(_ context.Context, _ *model.Project) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1", map[string]any{"name": &newName}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("PATCH /projects/{id} invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, rawRequest(http.MethodPatch, "/projects/proj-1", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /projects/{id} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		pr.UpdateFn = func(_ context.Context, _ *model.Project) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1", map[string]any{"name": "X"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteProject(t *testing.T) {
	t.Run("DELETE /projects/{id} success returns 204", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteFn = func(_ context.Context, _ string) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})

	t.Run("DELETE /projects/{id} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteFn = func(_ context.Context, _ string) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestListMembers(t *testing.T) {
	t.Run("GET /projects/{id}/members success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListMembersFn = func(_ context.Context, _ string) ([]*model.ProjectMember, error) {
			return []*model.ProjectMember{{ProjectID: "proj-1", UserID: "user-2", Role: model.RoleRead}}, nil
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1/members", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /projects/{id}/members repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListMembersFn = func(_ context.Context, _ string) ([]*model.ProjectMember, error) {
			return nil, errors.New("db error")
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1/members", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestAddMemberExtra(t *testing.T) {
	t.Run("POST /projects/{id}/members missing user_id returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/members", map[string]any{"user_id": "   ", "role": model.RoleRead}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /projects/{id}/members cannot add yourself returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/members", map[string]any{"user_id": "user-1", "role": model.RoleRead}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /projects/{id}/members repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddMemberFn = func(_ context.Context, _ *model.ProjectMember) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/members", map[string]any{"user_id": "user-2", "role": model.RoleRead}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestUpdateMember(t *testing.T) {
	t.Run("PATCH /projects/{id}/members/{userID} success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.UpdateMemberRoleFn = func(_ context.Context, _, _, _ string) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1/members/user-2", map[string]any{"role": model.RoleModify}))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("PATCH /projects/{id}/members/{userID} invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, rawRequest(http.MethodPatch, "/projects/proj-1/members/user-2", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /projects/{id}/members/{userID} invalid role returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1/members/user-2", map[string]any{"role": "owner"}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /projects/{id}/members/{userID} changing owner role returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1/members/user-1", map[string]any{"role": model.RoleRead}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("PATCH /projects/{id}/members/{userID} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.UpdateMemberRoleFn = func(_ context.Context, _, _, _ string) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPatch, "/projects/proj-1/members/user-2", map[string]any{"role": model.RoleRead}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestRemoveMember(t *testing.T) {
	t.Run("DELETE /projects/{id}/members/{userID} success returns 204", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.RemoveMemberFn = func(_ context.Context, _, _ string) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1/members/user-2", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})

	t.Run("DELETE /projects/{id}/members/{userID} removing owner returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1/members/user-1", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("DELETE /projects/{id}/members/{userID} repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.RemoveMemberFn = func(_ context.Context, _, _ string) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1/members/user-2", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestListStatuses(t *testing.T) {
	t.Run("GET /projects/{id}/statuses success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListStatusesFn = func(_ context.Context, _ string) ([]*model.ProjectStatus, error) {
			return []*model.ProjectStatus{{Status: "todo", Position: 0}}, nil
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1/statuses", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /projects/{id}/statuses repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		pr.ListStatusesFn = func(_ context.Context, _ string) ([]*model.ProjectStatus, error) {
			return nil, errors.New("db error")
		}
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1/statuses", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestAddStatus(t *testing.T) {
	t.Run("POST /projects/{id}/statuses success returns 201", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddStatusFn = func(_ context.Context, _, _ string) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/statuses", map[string]any{"status": "review"}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})

	t.Run("POST /projects/{id}/statuses invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, rawRequest(http.MethodPost, "/projects/proj-1/statuses", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /projects/{id}/statuses repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.AddStatusFn = func(_ context.Context, _, _ string) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/statuses", map[string]any{"status": "review"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteStatusExtra(t *testing.T) {
	t.Run("DELETE /projects/{id}/statuses/{status} success returns 204", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteStatusFn = func(_ context.Context, _, _ string) error { return nil }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1/statuses/done", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
	})

	t.Run("DELETE /projects/{id}/statuses/{status} internal error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleAdmin)
		pr.DeleteStatusFn = func(_ context.Context, _, _ string) error { return errors.New("db error") }
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodDelete, "/projects/proj-1/statuses/done", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestListTasks(t *testing.T) {
	t.Run("GET /projects/{id}/tasks success returns 200", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		tr := &mock.TaskRepo{
			ListChildrenFn: func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
				return []*model.Task{}, nil
			},
		}
		h := newHandler(pr, tr)
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1/tasks", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("GET /projects/{id}/tasks repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		tr := &mock.TaskRepo{
			ListChildrenFn: func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
				return nil, errors.New("db error")
			},
		}
		h := newHandler(pr, tr)
		w := serve(h, newRequest(http.MethodGet, "/projects/proj-1/tasks", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestCreateTaskExtra(t *testing.T) {
	t.Run("POST /projects/{id}/tasks with read role returns 403", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleRead)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/tasks", map[string]any{"name": "T"}))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("POST /projects/{id}/tasks invalid JSON returns 400", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		h := newHandler(pr, &mock.TaskRepo{})
		w := serve(h, rawRequest(http.MethodPost, "/projects/proj-1/tasks", `{bad`))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("POST /projects/{id}/tasks success returns 201", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		tr := &mock.TaskRepo{
			CreateFn: func(_ context.Context, _ *model.Task) error { return nil },
		}
		h := newHandler(pr, tr)
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/tasks", map[string]any{"name": "T"}))
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}
	})

	t.Run("POST /projects/{id}/tasks repo error returns 500", func(t *testing.T) {
		pr := projectRepoWithAccess(model.RoleModify)
		tr := &mock.TaskRepo{
			CreateFn: func(_ context.Context, _ *model.Task) error { return errors.New("db error") },
		}
		h := newHandler(pr, tr)
		w := serve(h, newRequest(http.MethodPost, "/projects/proj-1/tasks", map[string]any{"name": "T"}))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}
