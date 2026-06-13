//go:build integration

package projects_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestProjectTasksIntegration_ListRootOnly(t *testing.T) {
	env := httptestutil.NewEnv(t)
	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})

	parent := seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: env.User.ID, Name: "Parent"})
	child := seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: env.User.ID, Name: "Child", ParentID: &parent.ID})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/projects/" + p.ID + "/tasks", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tasks []*model.Task
	httptestutil.Decode(t, res, &tasks)

	ids := taskIDs(tasks)
	if !containsStr(ids, parent.ID) {
		t.Fatal("root task missing from list")
	}
	if containsStr(ids, child.ID) {
		t.Fatal("child task should not appear in root list")
	}
}

func TestProjectTasksIntegration_ListFilters(t *testing.T) {
	env := httptestutil.NewEnv(t)
	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})
	assignee := createUser(t, env, "u-assign1", "Assignee")

	seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: env.User.ID, Name: "Todo Task", Status: "todo"})
	inProg := seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: env.User.ID, Name: "InProg Task", Status: "in_progress", AssigneeID: &assignee.ID})

	t.Run("filter by status", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/projects/" + p.ID + "/tasks?status=in_progress", Body: nil, UserID: env.User.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var tasks []*model.Task
		httptestutil.Decode(t, res, &tasks)
		ids := taskIDs(tasks)
		if containsStr(ids, inProg.ID) && len(tasks) == 1 {
		} else if len(tasks) != 1 || !containsStr(ids, inProg.ID) {
			t.Fatalf("expected 1 in_progress task, got %d", len(tasks))
		}
	})

	t.Run("filter by assignee_id", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/projects/" + p.ID + "/tasks?assignee_id=" + assignee.ID, Body: nil, UserID: env.User.ID})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}

		var tasks []*model.Task
		httptestutil.Decode(t, res, &tasks)
		for _, t2 := range tasks {
			if t2.AssigneeID == nil || *t2.AssigneeID != assignee.ID {
				t.Fatalf("task %s has wrong assignee", t2.ID)
			}
		}
	})
}

func TestProjectTasksIntegration_ListFilterByTag(t *testing.T) {
	env := httptestutil.NewEnv(t)
	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: env.User.ID})

	task1 := seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: env.User.ID, Name: "Tagged"})
	_ = seed.Task(t, env.Store, seed.TaskInput{ProjectID: p.ID, OwnerID: env.User.ID, Name: "Untagged"})

	if err := env.Store.Tags.Add(context.Background(), task1.ID, "bug"); err != nil {
		t.Fatalf("add tag: %v", err)
	}

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/projects/" + p.ID + "/tasks?tag=bug", Body: nil, UserID: env.User.ID})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var tasks []*model.Task
	httptestutil.Decode(t, res, &tasks)
	ids := taskIDs(tasks)
	if !containsStr(ids, task1.ID) {
		t.Fatal("tagged task missing")
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 tagged task, got %d", len(tasks))
	}
}

func TestProjectTasksIntegration_Create(t *testing.T) {
	env := httptestutil.NewEnv(t)
	owner := env.User
	modifier := createUser(t, env, "u-modtask", "ModifierTask")
	reader := createUser(t, env, "u-readtask", "ReaderTask")
	outsider := createUser(t, env, "u-outtask", "OutsiderTask")

	p := seed.Project(t, env.Store, seed.ProjectInput{OwnerID: owner.ID})
	addMember(t, env, p.ID, modifier.ID, model.RoleModify)
	addMember(t, env, p.ID, reader.ID, model.RoleRead)

	t.Run("owner creates task", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name":        "My Task",
			"description": "task desc",
			"due_date":    "2026-08-01",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
		}

		var task model.Task
		httptestutil.Decode(t, res, &task)
		if task.Name != "My Task" {
			t.Fatalf("name = %q, want %q", task.Name, "My Task")
		}
		if task.Status != "todo" {
			t.Fatalf("default status = %q, want %q", task.Status, "todo")
		}
		if task.ProjectID != p.ID {
			t.Fatalf("project_id = %q, want %q", task.ProjectID, p.ID)
		}
	})

	t.Run("modifier creates task", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name": "Mod Task",
		}, UserID: modifier.ID})
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
		}
	})

	t.Run("reader forbidden", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name": "Should Fail",
		}, UserID: reader.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("no access forbidden", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name": "Should Fail",
		}, UserID: outsider.ID})
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("blank name 422", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name": "   ",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid JSON 400", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: `{bad`, UserID: owner.ID})
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("default status is todo", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name": "Default Status Task",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
		}

		var task model.Task
		httptestutil.Decode(t, res, &task)
		if task.Status != "todo" {
			t.Fatalf("status = %q, want %q", task.Status, "todo")
		}
	})

	t.Run("explicit status", func(t *testing.T) {
		res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPost, Path: "/projects/" + p.ID + "/tasks", Body: map[string]any{
			"name":   "Explicit Status",
			"status": "in_progress",
		}, UserID: owner.ID})
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusCreated)
		}

		var task model.Task
		httptestutil.Decode(t, res, &task)
		if task.Status != "in_progress" {
			t.Fatalf("status = %q, want %q", task.Status, "in_progress")
		}
	})
}

func taskIDs(tasks []*model.Task) []string {
	ids := make([]string, len(tasks))
	for i, t2 := range tasks {
		ids[i] = t2.ID
	}
	return ids
}
