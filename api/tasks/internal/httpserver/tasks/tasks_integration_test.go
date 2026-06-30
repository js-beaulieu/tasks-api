//go:build integration

package tasks_test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
	httptestutil "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/http"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/seed"
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

func TestTasksIntegration_Patch_CrossProjectMoveMovesSubtreeAndReturnsFallbacks(t *testing.T) {
	env := httptestutil.NewEnv(t)
	modifier := env.User
	targetOwner := seed.User(t, env.Store, seed.UserInput{ID: "target-owner", Name: "Target Owner", Email: "target@example.com"})
	preservedAssignee := seed.User(t, env.Store, seed.UserInput{ID: "member-keep", Name: "Keep Member", Email: "keep@example.com"})
	fallbackAssignee := seed.User(t, env.Store, seed.UserInput{ID: "member-drop", Name: "Drop Member", Email: "drop@example.com"})

	projectA := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: modifier.ID, Name: "A", AdditionalStatuses: []string{"review"}})
	projectB := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: targetOwner.ID, Name: "B"})
	if err := env.Store.Projects.AddMember(context.Background(), &model.ProjectMember{ProjectID: projectB.ID, UserID: modifier.ID, Role: model.RoleModify}); err != nil {
		t.Fatalf("AddMember modifier target project: %v", err)
	}
	if err := env.Store.Projects.AddMember(context.Background(), &model.ProjectMember{ProjectID: projectB.ID, UserID: preservedAssignee.ID, Role: model.RoleModify}); err != nil {
		t.Fatalf("AddMember preserved assignee target project: %v", err)
	}

	root := seed.Task(t, env.Store, seed.TaskInput{
		ProjectID:  projectA.ID,
		OwnerID:    modifier.ID,
		Status:     "review",
		AssigneeID: &fallbackAssignee.ID,
	})
	child := seed.Task(t, env.Store, seed.TaskInput{
		ProjectID:  projectA.ID,
		ParentID:   &root.ID,
		OwnerID:    modifier.ID,
		Status:     "done",
		AssigneeID: &preservedAssignee.ID,
	})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + root.ID, Body: map[string]any{
		"project_id": projectB.ID,
	}, UserID: modifier.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got model.Task
	httptestutil.Decode(t, res, &got)
	if got.ProjectID != projectB.ID {
		t.Fatalf("project_id = %q, want %q", got.ProjectID, projectB.ID)
	}
	if got.ParentID != nil {
		t.Fatalf("parent_id = %v, want nil", got.ParentID)
	}
	if got.Status != "todo" {
		t.Fatalf("status = %q, want todo fallback", got.Status)
	}
	if got.AssigneeID == nil || *got.AssigneeID != targetOwner.ID {
		t.Fatalf("assignee_id = %v, want target owner %q", got.AssigneeID, targetOwner.ID)
	}

	gotChild, err := env.Store.Tasks.Get(context.Background(), child.ID)
	if err != nil {
		t.Fatalf("Get child from DB: %v", err)
	}
	if gotChild.ProjectID != projectB.ID {
		t.Fatalf("child project_id = %q, want %q", gotChild.ProjectID, projectB.ID)
	}
	if gotChild.ParentID == nil || *gotChild.ParentID != root.ID {
		t.Fatalf("child parent_id = %v, want %q", gotChild.ParentID, root.ID)
	}
	if gotChild.AssigneeID == nil || *gotChild.AssigneeID != preservedAssignee.ID {
		t.Fatalf("child assignee_id = %v, want preserved assignee %q", gotChild.AssigneeID, preservedAssignee.ID)
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

func TestTasksIntegration_Delete_NotFound(t *testing.T) {
	env := httptestutil.NewEnv(t)
	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/nonexistent-id", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
}

func TestTasksIntegration_Delete_CompactsSiblingPositions(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	t0 := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	t1 := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})
	t2 := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodDelete, Path: "/tasks/" + t1.ID, Body: nil, UserID: env.User.ID})

	tasks, err := env.Store.Tasks.ListChildren(context.Background(), project.ID, nil, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	positions := map[string]int{}
	for _, tk := range tasks {
		positions[tk.ID] = tk.Position
	}
	if positions[t0.ID] != 0 {
		t.Errorf("t0 position = %d, want 0", positions[t0.ID])
	}
	if positions[t2.ID] != 1 {
		t.Errorf("t2 position = %d, want 1", positions[t2.ID])
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

func TestTasksIntegration_Patch_CompleteRecurring_ReturnsNextOccurrenceID(t *testing.T) {
	env := httptestutil.NewEnv(t)
	ctx := context.Background()
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	due := "2026-04-14"
	rec := "FREQ=DAILY"
	recurring := &model.Task{
		ProjectID:  project.ID,
		Name:       "Daily Task",
		Status:     "todo",
		DueDate:    &due,
		OwnerID:    env.User.ID,
		Recurrence: &rec,
	}
	if err := env.Store.Tasks.Create(ctx, recurring); err != nil {
		t.Fatalf("seed recurring task: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + recurring.ID, Body: map[string]any{
		"status": "done",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.StatusCode, http.StatusOK, res.Body)
	}

	nextID := res.Header.Get("X-Next-Occurrence-Id")
	if nextID == "" {
		t.Fatal("X-Next-Occurrence-Id header is empty, want a next occurrence ID")
	}

	// Verify the spawned task exists in the DB
	next, err := env.Store.Tasks.Get(ctx, nextID)
	if err != nil {
		t.Fatalf("Get next occurrence: %v", err)
	}
	if next.DueDate == nil || *next.DueDate != "2026-04-15" {
		t.Errorf("next due_date = %v, want 2026-04-15", next.DueDate)
	}
	if next.Status != "todo" {
		t.Errorf("next status = %q, want todo", next.Status)
	}
}

func TestTasksIntegration_Patch_CompleteNonRecurring_NoNextOccurrenceHeader(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	task := seed.Task(t, env.Store, seed.TaskInput{ProjectID: project.ID, OwnerID: env.User.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + task.ID, Body: map[string]any{
		"status": "done",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	nextID := res.Header.Get("X-Next-Occurrence-Id")
	if nextID != "" {
		t.Errorf("X-Next-Occurrence-Id header = %q, want empty for non-recurring task", nextID)
	}
}

func TestTasksIntegration_Patch_CompleteRecurringNoDueDate_Returns409(t *testing.T) {
	env := httptestutil.NewEnv(t)
	project := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	ctx := context.Background()
	rec := "FREQ=DAILY"
	recurring := &model.Task{
		ProjectID:  project.ID,
		Name:       "No Due Date",
		Status:     "todo",
		OwnerID:    env.User.ID,
		Recurrence: &rec,
	}
	if err := env.Store.Tasks.Create(ctx, recurring); err != nil {
		t.Fatalf("seed recurring task: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/tasks/" + recurring.ID, Body: map[string]any{
		"status": "done",
	}, UserID: env.User.ID})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusConflict)
	}
}
