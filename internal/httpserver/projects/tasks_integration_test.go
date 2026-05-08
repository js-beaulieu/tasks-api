//go:build integration

package projects_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestProjectTasksIntegration_CreateAndList(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})

	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/projects/"+project.ID+"/tasks", map[string]any{
		"name":        "Test Task",
		"description": "integration task",
		"status":      "todo",
		"due_date":    "2026-06-02",
	}, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusCreated)

	var task model.Task
	httptestutil.Decode(t, res, &task)

	res = httptestutil.Request(t, env.Handler, http.MethodGet, "/projects/"+project.ID+"/tasks", nil, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var tasks []*model.Task
	httptestutil.Decode(t, res, &tasks)
	if !containsTask(tasks, task.ID) {
		t.Fatalf("task %q not found in project task list", task.ID)
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
