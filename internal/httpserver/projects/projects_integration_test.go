//go:build integration

package projects_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestProjectsIntegration_CreateAndList(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env.Handler, http.MethodPost, "/projects", `{"name":"Test Project","description":"integration project","due_date":"2026-06-01","statuses":["review"]}`, env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusCreated)

	var project model.Project
	httptestutil.Decode(t, res, &project)

	res = httptestutil.Request(t, env.Handler, http.MethodGet, "/projects", "", env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var projects []*model.Project
	httptestutil.Decode(t, res, &projects)
	if !containsProject(projects, project.ID) {
		t.Fatalf("project %q not found in list", project.ID)
	}
}

func TestProjectsIntegration_ListStatuses(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, env.User.ID, "review")

	res := httptestutil.Request(t, env.Handler, http.MethodGet, "/projects/"+project.ID+"/statuses", "", env.User.ID)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var statuses []*model.ProjectStatus
	httptestutil.Decode(t, res, &statuses)
	want := []string{"todo", "in_progress", "done", "cancelled", "review"}
	if len(statuses) != len(want) {
		t.Fatalf("len(statuses) = %d, want %d", len(statuses), len(want))
	}
	for i, status := range statuses {
		if status.Status != want[i] {
			t.Fatalf("statuses[%d] = %q, want %q", i, status.Status, want[i])
		}
		if status.Position != i {
			t.Fatalf("statuses[%d].Position = %d, want %d", i, status.Position, i)
		}
	}
}

func containsProject(projects []*model.Project, id string) bool {
	for _, project := range projects {
		if project.ID == id {
			return true
		}
	}
	return false
}
