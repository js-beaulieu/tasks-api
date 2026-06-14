//go:build integration

package tasks_test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

// ── helpers ──────────────────────────────────────────────────────────────────

var userCounter int64

// nextUserID returns a unique user ID for test seed data.
func nextUserID() string {
	return fmt.Sprintf("u-test-%d", atomic.AddInt64(&userCounter, 1))
}

// createUserWithRole creates a new user with a unique ID and adds them to a project with the given role.
func createUserWithRole(t *testing.T, env *httptestutil.Env, projectID, role string) *model.User {
	t.Helper()
	u := seed.User(t, env.Store, seed.UserInput{ID: nextUserID(), Name: role + " user"})
	ctx := context.Background()
	if err := env.Store.Projects.AddMember(ctx, &model.ProjectMember{
		ProjectID: projectID,
		UserID:    u.ID,
		Role:      role,
	}); err != nil {
		t.Fatalf("add member role=%s: %v", role, err)
	}
	return u
}

// ── GET /tasks/{taskID} ─────────────────────────────────────────────────────

func TestTasksIntegration_Get_AuthorizedByOwner(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID, Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.ID != task.ID {
		t.Fatalf("got ID %q, want %q", got.ID, task.ID)
	}
}

func TestTasksIntegration_Get_ReadMemberCanAccess(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID, Body: nil, UserID: reader.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
}

func TestTasksIntegration_Get_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID, Body: nil, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_Get_MissingTaskNotFound(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/nonexistent-id", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
}

// ── PATCH /tasks/{taskID} ───────────────────────────────────────────────────

func TestTasksIntegration_Patch_OwnerUpdatesFields(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, Name: "original"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"name":   "updated",
		"status": "in_progress",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.Name != "updated" {
		t.Fatalf("Name = %q, want %q", got.Name, "updated")
	}
	if got.Status != "in_progress" {
		t.Fatalf("Status = %q, want %q", got.Status, "in_progress")
	}
}

func TestTasksIntegration_Patch_ModifyMemberUpdatesFields(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	modifier := createUserWithRole(t, env, project.ID, model.RoleModify)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"name": "modified by modify",
	}, UserID: modifier.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
}

func TestTasksIntegration_Patch_ReadForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"name": "try update",
	}, UserID: reader.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_Patch_InvalidJSON(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: `{invalid`, UserID: env.User.ID})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

func TestTasksIntegration_Patch_InvalidStatus409(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"status": "nonexistent_status",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusConflict)
	}
}

func TestTasksIntegration_Patch_ParentIDNullDetaches(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	child := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, ParentID: &parent.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + child.ID, Body: map[string]any{
		"parent_id": nil,
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.ParentID != nil {
		t.Fatalf("parent_id = %v, want nil", got.ParentID)
	}
}

func TestTasksIntegration_Patch_CrossProjectMoveWithModifyOnBoth(t *testing.T) {
	env := httptestutil.NewEnv(t)
	projectA := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID, Name: "A"})
	projectB := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID, Name: "B"})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: projectA.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"project_id": projectB.ID,
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.ProjectID != projectB.ID {
		t.Fatalf("project_id = %q, want %q", got.ProjectID, projectB.ID)
	}
}

func TestTasksIntegration_Patch_CrossProjectMoveForbiddenWithoutTargetModify(t *testing.T) {
	env := httptestutil.NewEnv(t)
	projectA := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID, Name: "A"})
	projectB := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID, Name: "B"})
	modifier := createUserWithRole(t, env, projectA.ID, model.RoleModify)
	_ = createUserWithRole(t, env, projectB.ID, model.RoleRead) // nolint:staticcheck // reader on target project
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: projectA.ID, OwnerID: modifier.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"project_id": projectB.ID,
	}, UserID: modifier.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_Patch_UpdateReflectsInDB(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, Name: "before"})

	httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"name": "after",
	}, UserID: env.User.ID})

	got, err := env.Store.Tasks.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task from DB: %v", err)
	}
	if got.Name != "after" {
		t.Fatalf("DB name = %q, want %q", got.Name, "after")
	}
}

// ── DELETE /tasks/{taskID} ───────────────────────────────────────────────────

func TestTasksIntegration_Delete_OwnerDeletes(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID, Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNoContent)
	}
}

func TestTasksIntegration_Delete_ModifyMemberDeletes(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	modifier := createUserWithRole(t, env, project.ID, model.RoleModify)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID, Body: nil, UserID: modifier.ID})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNoContent)
	}
}

func TestTasksIntegration_Delete_ReadForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID, Body: nil, UserID: reader.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_Delete_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID, Body: nil, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_Delete_RemovedFromDB(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID, Body: nil, UserID: env.User.ID})

	_, err := env.Store.Tasks.Get(context.Background(), task.ID)
	if err == nil {
		t.Fatal("task still in DB after delete")
	}
}

// ── POST /tasks/{taskID}/complete ────────────────────────────────────────────

func TestTasksIntegration_Complete_NonRecurring(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/complete", Body: map[string]any{
		"done_status": "done",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var body struct {
		Completed *model.Task `json:"completed"`
		Next      *model.Task `json:"next"`
	}
	httptestutil.Decode(t, res, &body)
	if body.Completed == nil || body.Completed.Status != "done" {
		t.Fatalf("completed = %#v, want status done", body.Completed)
	}
	if body.Next != nil {
		t.Fatalf("next = %#v, want nil for non-recurring", body.Next)
	}
}

func TestTasksIntegration_Complete_Recurring(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	ctx := context.Background()

	due := "2026-05-08"
	recurrence := "FREQ=DAILY"
	recurring := &model.Task{
		ProjectID:  project.ID,
		Name:       "Daily follow-up",
		Status:     "todo",
		DueDate:    &due,
		OwnerID:    env.User.ID,
		Recurrence: &recurrence,
	}
	if err := env.Store.Tasks.Create(ctx, recurring); err != nil {
		t.Fatalf("seed recurring task: %v", err)
	}
	if err := env.Store.Tags.Add(ctx, recurring.ID, "recurring"); err != nil {
		t.Fatalf("seed recurring tag: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + recurring.ID + "/complete", Body: map[string]any{"done_status": "done"}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var body struct {
		Completed *model.Task `json:"completed"`
		Next      *model.Task `json:"next"`
	}
	httptestutil.Decode(t, res, &body)
	if body.Completed == nil || body.Completed.Status != "done" {
		t.Fatalf("completed = %#v, want status done", body.Completed)
	}
	if body.Next == nil {
		t.Fatal("next = nil, want next occurrence")
	}
	if body.Next.DueDate == nil || *body.Next.DueDate != "2026-05-09" {
		t.Fatalf("next due_date = %v, want 2026-05-09", body.Next.DueDate)
	}
	if body.Next.Status != "todo" {
		t.Fatalf("next status = %q, want todo", body.Next.Status)
	}

	tags, err := env.Store.Tags.ListForTask(ctx, body.Next.ID)
	if err != nil {
		t.Fatalf("list next tags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "recurring" {
		t.Fatalf("next tags = %v, want [recurring]", tags)
	}
}

func TestTasksIntegration_Complete_ReadForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/complete", Body: map[string]any{
		"done_status": "done",
	}, UserID: reader.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_Complete_BlankDoneStatus(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/complete", Body: map[string]any{
		"done_status": "",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestTasksIntegration_Complete_InvalidDoneStatus(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/complete", Body: map[string]any{
		"done_status": "nonexistent",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusConflict)
	}
}

func TestTasksIntegration_Complete_InvalidJSON(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/complete", Body: `{invalid`, UserID: env.User.ID})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

// ── GET /tasks/{taskID}/tasks ────────────────────────────────────────────────

func TestTasksIntegration_ListSubtasks_DirectChildrenOnly(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	child := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, ParentID: &parent.ID})
	_ = seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, ParentID: &child.ID}) // grandchild
	_ = seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})                      // sibling of parent

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + parent.ID + "/tasks", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tasks []*model.Task
	httptestutil.Decode(t, res, &tasks)
	if len(tasks) != 1 || tasks[0].ID != child.ID {
		t.Fatalf("subtasks = %d tasks, want exactly child %q (no grandchildren/siblings)", len(tasks), child.ID)
	}
}

func TestTasksIntegration_ListSubtasks_ReadCanList(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	_ = seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, ParentID: &parent.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + parent.ID + "/tasks", Body: nil, UserID: reader.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
}

func TestTasksIntegration_ListSubtasks_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + parent.ID + "/tasks", Body: nil, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

// ── POST /tasks/{taskID}/tasks ──────────────────────────────────────────────

func TestTasksIntegration_CreateSubtask_OwnerCreates(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "Subtask",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	var sub model.Task
	httptestutil.Decode(t, res, &sub)
	if sub.ProjectID != project.ID {
		t.Fatalf("subtask project_id = %q, want %q", sub.ProjectID, project.ID)
	}
	if sub.ParentID == nil || *sub.ParentID != parent.ID {
		t.Fatalf("subtask parent_id = %v, want %q", sub.ParentID, parent.ID)
	}
	if sub.OwnerID != env.User.ID {
		t.Fatalf("subtask owner_id = %q, want %q", sub.OwnerID, env.User.ID)
	}
	if sub.Status != "todo" {
		t.Fatalf("subtask status = %q, want todo", sub.Status)
	}
}

func TestTasksIntegration_CreateSubtask_ModifyMemberCreates(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	modifier := createUserWithRole(t, env, project.ID, model.RoleModify)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "Sub by modifier",
	}, UserID: modifier.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}
}

func TestTasksIntegration_CreateSubtask_ReadForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "forbidden subtask",
	}, UserID: reader.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_CreateSubtask_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "forbidden subtask",
	}, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_CreateSubtask_BlankName(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestTasksIntegration_CreateSubtask_InvalidJSON(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: `{invalid`, UserID: env.User.ID})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

// ── GET /tasks/{taskID}/tags ─────────────────────────────────────────────────

func TestTasksIntegration_ListTags_EmptyArray(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID + "/tags", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tags []string
	httptestutil.Decode(t, res, &tags)
	if tags == nil {
		t.Fatal("tags = nil, want empty array")
	}
	if len(tags) != 0 {
		t.Fatalf("tags = %v, want empty array", tags)
	}
}

func TestTasksIntegration_ListTags_ReturnsTags(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	ctx := context.Background()
	if err := env.Store.Tags.Add(ctx, task.ID, "alpha"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := env.Store.Tags.Add(ctx, task.ID, "beta"); err != nil {
		t.Fatalf("add tag: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID + "/tags", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tags []string
	httptestutil.Decode(t, res, &tags)
	if len(tags) != 2 {
		t.Fatalf("len(tags) = %d, want 2", len(tags))
	}
}

func TestTasksIntegration_ListTags_ReadCanList(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID + "/tags", Body: nil, UserID: reader.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
}

func TestTasksIntegration_ListTags_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/tasks/" + task.ID + "/tags", Body: nil, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

// ── POST /tasks/{taskID}/tags ────────────────────────────────────────────────

func TestTasksIntegration_AddTag_OwnerAdds(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{
		"tag": "important",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["tag"] != "important" {
		t.Fatalf("tag = %q, want %q", body["tag"], "important")
	}

	tags, err := env.Store.Tags.ListForTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	found := false
	for _, tg := range tags {
		if tg == "important" {
			found = true
		}
	}
	if !found {
		t.Fatal("tag 'important' not found in DB")
	}
}

func TestTasksIntegration_AddTag_ModifyMemberAdds(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	modifier := createUserWithRole(t, env, project.ID, model.RoleModify)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{
		"tag": "by-modifier",
	}, UserID: modifier.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
	}
}

func TestTasksIntegration_AddTag_ReadForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{
		"tag": "blocked",
	}, UserID: reader.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_AddTag_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{
		"tag": "blocked",
	}, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_AddTag_BlankTag(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: map[string]any{
		"tag": "",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestTasksIntegration_AddTag_InvalidJSON(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + task.ID + "/tags", Body: `{invalid`, UserID: env.User.ID})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

// ── DELETE /tasks/{taskID}/tags/{tag} ────────────────────────────────────────

func TestTasksIntegration_DeleteTag_OwnerDeletes(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	ctx := context.Background()
	if err := env.Store.Tags.Add(ctx, task.ID, "remove-me"); err != nil {
		t.Fatalf("add tag: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID + "/tags/remove-me", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNoContent)
	}

	tags, _ := env.Store.Tags.ListForTask(ctx, task.ID)
	for _, tg := range tags {
		if tg == "remove-me" {
			t.Fatal("tag 'remove-me' still present after delete")
		}
	}
}

func TestTasksIntegration_DeleteTag_ModifyMemberDeletes(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	ctx := context.Background()
	if err := env.Store.Tags.Add(ctx, task.ID, "deleteme"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	modifier := createUserWithRole(t, env, project.ID, model.RoleModify)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID + "/tags/deleteme", Body: nil, UserID: modifier.ID})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNoContent)
	}
}

func TestTasksIntegration_DeleteTag_ReadForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	ctx := context.Background()
	_ = env.Store.Tags.Add(ctx, task.ID, "keep")
	reader := createUserWithRole(t, env, project.ID, model.RoleRead)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID + "/tags/keep", Body: nil, UserID: reader.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestTasksIntegration_DeleteTag_NoAccessForbidden(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	outsider := seed.User(t, env.Store, seed.UserInput{Name: "outsider"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + task.ID + "/tags/any", Body: nil, UserID: outsider.ID})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

// ── Recurrence editing ─────────────────────────────────────────────────────

func TestTasksIntegration_Patch_SetRecurrence(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID, DueDate: strPtr("2026-06-01")})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"recurrence": "FREQ=DAILY",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.StatusCode, http.StatusOK, res.Body)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.Recurrence == nil || *got.Recurrence != "FREQ=DAILY" {
		t.Fatalf("recurrence = %v, want FREQ=DAILY", got.Recurrence)
	}
}

func TestTasksIntegration_Patch_ClearRecurrence(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	ctx := context.Background()
	due := "2026-06-01"
	recurrence := "FREQ=WEEKLY"
	recurring := &model.Task{
		ProjectID:  project.ID,
		Name:       "Recurring",
		Status:     "todo",
		DueDate:    &due,
		OwnerID:    env.User.ID,
		Recurrence: &recurrence,
	}
	if err := env.Store.Tasks.Create(ctx, recurring); err != nil {
		t.Fatalf("seed recurring task: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + recurring.ID, Body: map[string]any{
		"recurrence": nil,
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.StatusCode, http.StatusOK, res.Body)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.Recurrence != nil {
		t.Fatalf("recurrence = %v, want nil", got.Recurrence)
	}

	fresh, err := env.Store.Tasks.Get(ctx, recurring.ID)
	if err != nil {
		t.Fatalf("re-fetch task: %v", err)
	}
	if fresh.Recurrence != nil {
		t.Fatalf("recurrence persisted = %v, want nil", fresh.Recurrence)
	}
}

func TestTasksIntegration_Patch_InvalidRecurrence(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"recurrence": "INVALID",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestTasksIntegration_Patch_RecurrenceWithoutDueDate(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"recurrence": "FREQ=DAILY",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestTasksIntegration_CreateSubtask_WithRecurrence(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "Recurring sub", "recurrence": "FREQ=DAILY", "due_date": "2026-06-01",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", res.StatusCode, http.StatusCreated, res.Body)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.Recurrence == nil || *got.Recurrence != "FREQ=DAILY" {
		t.Fatalf("recurrence = %v, want FREQ=DAILY", got.Recurrence)
	}
}

func TestTasksIntegration_CreateSubtask_RecurrenceWithoutDueDate(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/tasks/" + parent.ID + "/tasks", Body: map[string]any{
		"name": "No due", "recurrence": "FREQ=DAILY",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func strPtr(s string) *string { return &s }
