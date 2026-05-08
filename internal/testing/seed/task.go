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

// Task creates a task through the supplied test target and fatals on error.
// Supported targets:
//   - *postgres.Store: Task(t, store, projectID, ownerID, parentID)
//   - *httptestutil.Env: Task(t, env, projectID)
//   - *httptestutil.Env: Task(t, env, parentID, true)
//   - *mcptest.Env: Task(t, env, projectID)
//   - *mcptest.Env: Task(t, env, parentTask)
func Task(t *testing.T, target any, args ...any) *model.Task {
	t.Helper()

	switch v := target.(type) {
	case *postgres.Store:
		projectID := arg[string](t, args, 0, "projectID")
		ownerID := arg[string](t, args, 1, "ownerID")
		parentID := arg[*string](t, args, 2, "parentID")
		task := &model.Task{
			ProjectID: projectID,
			ParentID:  parentID,
			Name:      "Test Task",
			OwnerID:   ownerID,
			Status:    "todo",
		}
		if err := v.Tasks.Create(context.Background(), task); err != nil {
			t.Fatalf("seed.Task: %v", err)
		}
		return task
	case *httptestutil.Env:
		if isSubtaskSeed(args) {
			parentID := arg[string](t, args, 0, "parentID")
			res := httptestutil.Request(t, v.Handler, http.MethodPost, "/tasks/"+parentID+"/tasks", `{"name":"Test Task","status":"todo"}`, v.User.ID)
			httptestutil.AssertStatus(t, res, http.StatusCreated)

			var task model.Task
			httptestutil.Decode(t, res, &task)
			return &task
		}

		projectID := arg[string](t, args, 0, "projectID")
		body := `{"name":"Test Task","description":"integration task","status":"todo","due_date":"2026-06-02"}`
		res := httptestutil.Request(t, v.Handler, http.MethodPost, "/projects/"+projectID+"/tasks", body, v.User.ID)
		httptestutil.AssertStatus(t, res, http.StatusCreated)

		var task model.Task
		httptestutil.Decode(t, res, &task)
		return &task
	case *mcptest.Env:
		if parent, ok := optionalArg[*model.Task](args, 0); ok {
			result := mcptest.CallTool(t, v, "create_task", map[string]any{
				"project_id": parent.ProjectID,
				"parent_id":  parent.ID,
				"name":       "Test Task",
				"status":     "todo",
			})
			task := mcptest.DecodeStructured[model.Task](t, result)
			return &task
		}

		projectID := arg[string](t, args, 0, "projectID")
		result := mcptest.CallTool(t, v, "create_task", map[string]any{
			"project_id":  projectID,
			"name":        "Test Task",
			"description": "integration task",
			"status":      "todo",
			"due_date":    "2026-06-02",
		})
		task := mcptest.DecodeStructured[model.Task](t, result)
		return &task
	default:
		t.Fatalf("seed.Task unsupported target %T", target)
		return nil
	}
}

func isSubtaskSeed(args []any) bool {
	isSubtask, ok := optionalArg[bool](args, 1)
	return ok && isSubtask
}
