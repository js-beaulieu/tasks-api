package httptestutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/config"
	"github.com/js-beaulieu/tasks-api/internal/httpserver"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

type Env struct {
	Store   *postgres.Store
	Handler http.Handler
	User    *model.User
}

func NewEnv(t *testing.T) *Env {
	t.Helper()

	_, store := testdb.Open(t)
	user := seed.User(t, store, "u-http-1", "HTTP User", "http-user@example.com")
	return &Env{
		Store:   store,
		Handler: httpserver.New(store, config.Config{}),
		User:    user,
	}
}

func Request(t *testing.T, handler http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
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

func AssertStatus(t *testing.T, res *httptest.ResponseRecorder, want int) {
	t.Helper()

	if res.Code != want {
		t.Fatalf("status = %d, want %d, body: %s", res.Code, want, res.Body.String())
	}
}

func Decode(t *testing.T, res *httptest.ResponseRecorder, v any) {
	t.Helper()

	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, res.Body.String())
	}
}

func CreateProject(t *testing.T, env *Env) *model.Project {
	t.Helper()

	body := `{"name":"HTTP Project","description":"integration project","due_date":"2026-06-01","statuses":["review"]}`
	res := Request(t, env.Handler, http.MethodPost, "/projects", body, env.User.ID)
	AssertStatus(t, res, http.StatusCreated)

	var project model.Project
	Decode(t, res, &project)
	if project.ID == "" {
		t.Fatal("project ID is empty")
	}
	if project.Name != "HTTP Project" {
		t.Fatalf("project name = %q, want HTTP Project", project.Name)
	}
	return &project
}

func CreateTask(t *testing.T, env *Env, projectID string) *model.Task {
	t.Helper()

	body := `{"name":"HTTP Task","description":"integration task","status":"todo","due_date":"2026-06-02"}`
	res := Request(t, env.Handler, http.MethodPost, "/projects/"+projectID+"/tasks", body, env.User.ID)
	AssertStatus(t, res, http.StatusCreated)

	var task model.Task
	Decode(t, res, &task)
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

func CreateSubtask(t *testing.T, env *Env, parentID string) *model.Task {
	t.Helper()

	res := Request(t, env.Handler, http.MethodPost, "/tasks/"+parentID+"/tasks", `{"name":"HTTP Subtask","status":"todo"}`, env.User.ID)
	AssertStatus(t, res, http.StatusCreated)

	var task model.Task
	Decode(t, res, &task)
	if task.ParentID == nil || *task.ParentID != parentID {
		t.Fatalf("subtask parent_id = %v, want %q", task.ParentID, parentID)
	}
	return &task
}
