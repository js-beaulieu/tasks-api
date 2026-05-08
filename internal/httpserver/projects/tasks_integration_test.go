//go:build integration

package projects_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/httptestutil"
	"github.com/js-beaulieu/tasks-api/internal/model"
)

func TestProjectTasksIntegration_CreateAndList(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := httptestutil.CreateProject(t, env)
	task := httptestutil.CreateTask(t, env, project.ID)

	res := httptestutil.Request(t, env.Handler, http.MethodGet, "/projects/"+project.ID+"/tasks", "", env.User.ID)
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
