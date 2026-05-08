//go:build integration

package tags_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestTagsIntegration_AddAndListForTask(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/tasks/"+task.ID+"/tags", map[string]any{"tag": "backend"}, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusCreated)

	res = httptestutil.Request(t, env.Handler, http.MethodGet, "/tasks/"+task.ID+"/tags", nil, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var tags []string
	httptestutil.Decode(t, res, &tags)
	if len(tags) != 1 || tags[0] != "backend" {
		t.Fatalf("tags = %v, want [backend]", tags)
	}
}
