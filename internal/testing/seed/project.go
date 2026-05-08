package seed

import (
	"context"
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
	mcptest "github.com/js-beaulieu/tasks-api/internal/testing/mcp"
)

// Project creates a project through the supplied test target and fatals on error.
// Supported targets:
//   - *postgres.Store: Project(t, store, ownerID)
//   - *httptestutil.Env: Project(t, env)
//   - *mcptest.Env: Project(t, env)
func Project(t *testing.T, target any, args ...any) *model.Project {
	t.Helper()

	switch v := target.(type) {
	case *postgres.Store:
		ownerID := arg[string](t, args, 0, "ownerID")
		p := &model.Project{Name: "Test Project", OwnerID: ownerID}
		if err := v.Projects.Create(context.Background(), p); err != nil {
			t.Fatalf("seed.Project: %v", err)
		}
		return p
	case *httptestutil.Env:
		body := `{"name":"Test Project","description":"integration project","due_date":"2026-06-01","statuses":["review"]}`
		res := httptestutil.Request(t, v.Handler, http.MethodPost, "/projects", body, v.User.ID)
		httptestutil.AssertStatus(t, res, http.StatusCreated)

		var project model.Project
		httptestutil.Decode(t, res, &project)
		return &project
	case *mcptest.Env:
		result := mcptest.CallTool(t, v, "create_project", map[string]any{
			"name":        "Test Project",
			"description": "integration project",
			"due_date":    "2026-06-01",
			"statuses":    []string{"review"},
		})
		project := mcptest.DecodeStructured[model.Project](t, result)
		return &project
	default:
		t.Fatalf("seed.Project unsupported target %T", target)
		return nil
	}
}
