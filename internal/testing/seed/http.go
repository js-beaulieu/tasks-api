package seed

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
)

func HTTPProject(t *testing.T, env *httptestutil.Env) *model.Project {
	t.Helper()

	body := `{"name":"HTTP Project","description":"integration project","due_date":"2026-06-01","statuses":["review"]}`
	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/projects", body, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusCreated)

	var project model.Project
	httptestutil.Decode(t, res, &project)
	return &project
}

func HTTPTask(t *testing.T, env *httptestutil.Env, projectID string) *model.Task {
	t.Helper()

	body := `{"name":"HTTP Task","description":"integration task","status":"todo","due_date":"2026-06-02"}`
	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/projects/"+projectID+"/tasks", body, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusCreated)

	var task model.Task
	httptestutil.Decode(t, res, &task)
	return &task
}

func HTTPSubtask(t *testing.T, env *httptestutil.Env, parentID string) *model.Task {
	t.Helper()

	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/tasks/"+parentID+"/tasks", `{"name":"HTTP Subtask","status":"todo"}`, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusCreated)

	var task model.Task
	httptestutil.Decode(t, res, &task)
	return &task
}
