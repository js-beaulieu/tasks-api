//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

// ---- Create / Get ----

func TestTasks_CreateGet(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	task := &model.Task{
		ProjectID: proj.ID,
		Name:      "First Task",
		OwnerID:   owner.ID,
		Status:    "todo",
	}
	if err := store.Tasks.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == "" {
		t.Fatal("Create did not assign ID")
	}

	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "First Task" {
		t.Errorf("Name = %q, want %q", got.Name, "First Task")
	}
	if got.ProjectID != proj.ID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, proj.ID)
	}
	if got.Position != 0 {
		t.Errorf("Position = %d, want 0 (first task)", got.Position)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestTasks_Get_NotFound(t *testing.T) {
	_, store := testdb.Open(t)
	_, err := store.Tasks.Get(context.Background(), "no-such-id")
	if err != repo.ErrNotFound {
		t.Errorf("err = %v, want repo.ErrNotFound", err)
	}
}

func TestTasks_Create_PositionAutoIncrement(t *testing.T) {
	_, store := testdb.Open(t)
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	if t1.Position != 0 {
		t.Errorf("t1.Position = %d, want 0", t1.Position)
	}
	if t2.Position != 1 {
		t.Errorf("t2.Position = %d, want 1", t2.Position)
	}
}

func TestTasks_Create_InvalidStatus(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	task := &model.Task{
		ProjectID: proj.ID,
		Name:      "Bad Status",
		OwnerID:   owner.ID,
		Status:    "no_such_status",
	}
	err := store.Tasks.Create(ctx, task)
	if err != repo.ErrConflict {
		t.Errorf("err = %v, want repo.ErrConflict for invalid status", err)
	}
}

func TestTasks_Create_CustomStatus(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	if err := store.Projects.AddStatus(ctx, proj.ID, "review"); err != nil {
		t.Fatalf("AddStatus: %v", err)
	}

	task := &model.Task{
		ProjectID: proj.ID,
		Name:      "Review Task",
		OwnerID:   owner.ID,
		Status:    "review",
	}
	if err := store.Tasks.Create(ctx, task); err != nil {
		t.Errorf("Create with custom status: %v", err)
	}
}

// ---- ListChildren ----

func TestTasks_ListChildren(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	t.Run("top-level tasks returned ordered by position", func(t *testing.T) {
		tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{})
		if err != nil {
			t.Fatalf("ListChildren: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("len = %d, want 2", len(tasks))
		}
		if tasks[0].ID != t1.ID || tasks[1].ID != t2.ID {
			t.Errorf("order wrong: got [%s,%s], want [%s,%s]", tasks[0].ID, tasks[1].ID, t1.ID, t2.ID)
		}
	})

	t.Run("status filter returns only matching tasks", func(t *testing.T) {
		// t1 is todo; t2 is todo — update t2 to in_progress for filtering
		t2.Status = "in_progress"
		if _, _, err := store.Tasks.Update(ctx, t2); err != nil {
			t.Fatalf("Update: %v", err)
		}
		status := "in_progress"
		tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &status})
		if err != nil {
			t.Fatalf("ListChildren with status filter: %v", err)
		}
		if len(tasks) != 1 || tasks[0].ID != t2.ID {
			t.Errorf("expected only t2, got %d tasks", len(tasks))
		}
	})

	t.Run("assignee filter returns only matching tasks", func(t *testing.T) {
		assignee := seed.User(t, store, seed.UserInput{ID: "u2", Name: "Bob", Email: "bob@test.com"})
		t1.AssigneeID = &assignee.ID
		if _, _, err := store.Tasks.Update(ctx, t1); err != nil {
			t.Fatalf("Update: %v", err)
		}
		tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{AssigneeID: &assignee.ID})
		if err != nil {
			t.Fatalf("ListChildren with assignee filter: %v", err)
		}
		if len(tasks) != 1 || tasks[0].ID != t1.ID {
			t.Errorf("expected only t1, got %d tasks", len(tasks))
		}
	})

	t.Run("tag filter returns only tasks with that tag", func(t *testing.T) {
		if err := store.Tags.Add(ctx, t1.ID, "urgent"); err != nil {
			t.Fatalf("Add tag: %v", err)
		}
		tag := "urgent"
		tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Tag: &tag})
		if err != nil {
			t.Fatalf("ListChildren with tag filter: %v", err)
		}
		if len(tasks) != 1 || tasks[0].ID != t1.ID {
			t.Errorf("expected only t1, got %d tasks", len(tasks))
		}
	})

	t.Run("status+tag combined filter", func(t *testing.T) {
		// t1 has tag "urgent" and status "todo"; t2 has status "in_progress"
		// Filter: status=todo AND tag=urgent → only t1
		status := "todo"
		tag := "urgent"
		tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &status, Tag: &tag})
		if err != nil {
			t.Fatalf("ListChildren combined filter: %v", err)
		}
		if len(tasks) != 1 || tasks[0].ID != t1.ID {
			t.Errorf("expected only t1, got %d tasks", len(tasks))
		}
	})
}

// ---- Update ----

func TestTasks_Update_NameOnly(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})
	task := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	originalUpdatedAt := task.UpdatedAt
	task.Name = "Updated Name"
	if _, _, err := store.Tasks.Update(ctx, task); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Name")
	}
	if !got.UpdatedAt.After(originalUpdatedAt) {
		t.Errorf("UpdatedAt not refreshed: was %v, got %v", originalUpdatedAt, got.UpdatedAt)
	}
}

func TestTasks_Update_InvalidStatus(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})
	task := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	task.Status = "bogus"
	_, _, err := store.Tasks.Update(ctx, task)
	if err != repo.ErrConflict {
		t.Errorf("err = %v, want repo.ErrConflict for invalid status", err)
	}
}

func TestTasks_Update_PositionReorderUp(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID}) // pos 0
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID}) // pos 1
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID}) // pos 2

	// Move t2 (pos=2) up to pos=0
	t2.Position = 0
	if _, _, err := store.Tasks.Update(ctx, t2); err != nil {
		t.Fatalf("Update: %v", err)
	}

	tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("len = %d, want 3", len(tasks))
	}

	positions := map[string]int{}
	for _, tk := range tasks {
		positions[tk.ID] = tk.Position
	}
	if positions[t2.ID] != 0 {
		t.Errorf("t2 position = %d, want 0", positions[t2.ID])
	}
	if positions[t0.ID] != 1 {
		t.Errorf("t0 position = %d, want 1 (shifted down)", positions[t0.ID])
	}
	if positions[t1.ID] != 2 {
		t.Errorf("t1 position = %d, want 2 (shifted down)", positions[t1.ID])
	}
}

func TestTasks_Update_PositionReorderDown(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID}) // pos 0
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID}) // pos 1
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID}) // pos 2

	// Move t0 (pos=0) down to pos=2
	t0.Position = 2
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	tasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}

	positions := map[string]int{}
	for _, tk := range tasks {
		positions[tk.ID] = tk.Position
	}
	if positions[t0.ID] != 2 {
		t.Errorf("t0 position = %d, want 2", positions[t0.ID])
	}
	if positions[t1.ID] != 0 {
		t.Errorf("t1 position = %d, want 0 (shifted up)", positions[t1.ID])
	}
	if positions[t2.ID] != 1 {
		t.Errorf("t2 position = %d, want 1 (shifted up)", positions[t2.ID])
	}
}

// ---- Delete ----

func TestTasks_Delete(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})
	parent := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	child := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, ParentID: &parent.ID})

	if err := store.Tasks.Delete(ctx, parent.ID); err != nil {
		t.Fatalf("Delete parent: %v", err)
	}

	_, err := store.Tasks.Get(ctx, parent.ID)
	if err != repo.ErrNotFound {
		t.Errorf("parent: err = %v, want repo.ErrNotFound", err)
	}
	_, err = store.Tasks.Get(ctx, child.ID)
	if err != repo.ErrNotFound {
		t.Errorf("child (cascade): err = %v, want repo.ErrNotFound", err)
	}
}

func TestTasks_Delete_CompactsPositions(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	if err := store.Tasks.Delete(ctx, t1.ID); err != nil {
		t.Fatalf("Delete middle: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t0.ID: 0,
		t2.ID: 1,
	})
}

func TestTasks_Delete_NotFound(t *testing.T) {
	_, store := testdb.Open(t)
	err := store.Tasks.Delete(context.Background(), "no-such-id")
	if err != repo.ErrNotFound {
		t.Errorf("err = %v, want repo.ErrNotFound", err)
	}
}

// ---- Move between parents (same project) ----

func TestTasks_Move_BetweenParents(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	parentA := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	parentB := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	child := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, ParentID: &parentA.ID})

	// Move child from parentA to parentB
	child.ParentID = &parentB.ID
	child.Position = 0
	if _, _, err := store.Tasks.Update(ctx, child); err != nil {
		t.Fatalf("Update (move): %v", err)
	}

	childrenA, err := store.Tasks.ListChildren(ctx, proj.ID, &parentA.ID, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren parentA: %v", err)
	}
	if len(childrenA) != 0 {
		t.Errorf("parentA children = %d, want 0", len(childrenA))
	}

	childrenB, err := store.Tasks.ListChildren(ctx, proj.ID, &parentB.ID, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren parentB: %v", err)
	}
	if len(childrenB) != 1 || childrenB[0].ID != child.ID {
		t.Errorf("parentB children wrong: got %d tasks", len(childrenB))
	}
	if childrenB[0].Position != 0 {
		t.Errorf("moved task position = %d, want 0", childrenB[0].Position)
	}
}

// ---- Move between projects ----

func TestTasks_Move_BetweenProjects(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	projX := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})
	projY := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	task := seed.Task(t, store, seed.TaskInput{ProjectID: projX.ID, OwnerID: owner.ID})
	// Extra task in projX to verify position compaction
	_ = seed.Task(t, store, seed.TaskInput{ProjectID: projX.ID, OwnerID: owner.ID})

	// Move task from projX to projY
	task.ProjectID = projY.ID
	task.ParentID = nil
	task.Position = 0
	if _, _, err := store.Tasks.Update(ctx, task); err != nil {
		t.Fatalf("Update (cross-project move): %v", err)
	}

	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ProjectID != projY.ID {
		t.Errorf("project_id = %q, want %q", got.ProjectID, projY.ID)
	}
	if got.ParentID != nil {
		t.Errorf("parent_id = %v, want nil", got.ParentID)
	}

	// projX should still have the second task at position 0 (compacted)
	tasksX, err := store.Tasks.ListChildren(ctx, projX.ID, nil, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren projX: %v", err)
	}
	if len(tasksX) != 1 {
		t.Fatalf("projX task count = %d, want 1", len(tasksX))
	}
	if tasksX[0].Position != 0 {
		t.Errorf("remaining projX task position = %d, want 0 (compacted)", tasksX[0].Position)
	}
}

func TestTasks_Move_SubtreeBetweenProjects(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	sourceOwner := seed.User(t, store, seed.UserInput{ID: "u-source", Name: "Alice", Email: "alice@test.com"})
	targetOwner := seed.User(t, store, seed.UserInput{ID: "u-target", Name: "Bob", Email: "bob@test.com"})
	preservedAssignee := seed.User(t, store, seed.UserInput{ID: "u-keep", Name: "Cara", Email: "cara@test.com"})
	fallbackAssignee := seed.User(t, store, seed.UserInput{ID: "u-fallback", Name: "Dan", Email: "dan@test.com"})

	projSource := seed.Project(t, store, seed.ProjectInput{OwnerID: sourceOwner.ID, AdditionalStatuses: []string{"review"}})
	projTarget := seed.Project(t, store, seed.ProjectInput{OwnerID: targetOwner.ID})
	if err := store.Projects.AddMember(ctx, &model.ProjectMember{ProjectID: projTarget.ID, UserID: preservedAssignee.ID, Role: model.RoleModify}); err != nil {
		t.Fatalf("AddMember target preserved assignee: %v", err)
	}

	oldParent := seed.Task(t, store, seed.TaskInput{ProjectID: projSource.ID, OwnerID: sourceOwner.ID})
	movedRoot := seed.Task(t, store, seed.TaskInput{
		ProjectID:  projSource.ID,
		ParentID:   &oldParent.ID,
		OwnerID:    sourceOwner.ID,
		Status:     "review",
		AssigneeID: &fallbackAssignee.ID,
	})
	child := seed.Task(t, store, seed.TaskInput{
		ProjectID:  projSource.ID,
		ParentID:   &movedRoot.ID,
		OwnerID:    sourceOwner.ID,
		Status:     "done",
		AssigneeID: &preservedAssignee.ID,
	})
	grandchild := seed.Task(t, store, seed.TaskInput{
		ProjectID: projSource.ID,
		ParentID:  &child.ID,
		OwnerID:   sourceOwner.ID,
		Status:    "review",
	})
	sourceSibling := seed.Task(t, store, seed.TaskInput{ProjectID: projSource.ID, ParentID: &oldParent.ID, OwnerID: sourceOwner.ID})
	targetExisting := seed.Task(t, store, seed.TaskInput{ProjectID: projTarget.ID, OwnerID: targetOwner.ID})

	movedRoot.ProjectID = projTarget.ID
	movedRoot.Position = 1
	if _, _, err := store.Tasks.Update(ctx, movedRoot); err != nil {
		t.Fatalf("Update (cross-project subtree move): %v", err)
	}

	gotRoot, err := store.Tasks.Get(ctx, movedRoot.ID)
	if err != nil {
		t.Fatalf("Get root: %v", err)
	}
	if gotRoot.ProjectID != projTarget.ID {
		t.Fatalf("root project_id = %q, want %q", gotRoot.ProjectID, projTarget.ID)
	}
	if gotRoot.ParentID != nil {
		t.Fatalf("root parent_id = %v, want nil", gotRoot.ParentID)
	}
	if gotRoot.Status != "todo" {
		t.Fatalf("root status = %q, want todo fallback", gotRoot.Status)
	}
	if gotRoot.AssigneeID == nil || *gotRoot.AssigneeID != targetOwner.ID {
		t.Fatalf("root assignee = %v, want target owner %q", gotRoot.AssigneeID, targetOwner.ID)
	}
	if gotRoot.Position != 1 {
		t.Fatalf("root position = %d, want 1", gotRoot.Position)
	}

	gotChild, err := store.Tasks.Get(ctx, child.ID)
	if err != nil {
		t.Fatalf("Get child: %v", err)
	}
	if gotChild.ProjectID != projTarget.ID {
		t.Fatalf("child project_id = %q, want %q", gotChild.ProjectID, projTarget.ID)
	}
	if gotChild.ParentID == nil || *gotChild.ParentID != movedRoot.ID {
		t.Fatalf("child parent_id = %v, want %q", gotChild.ParentID, movedRoot.ID)
	}
	if gotChild.Status != "done" {
		t.Fatalf("child status = %q, want done", gotChild.Status)
	}
	if gotChild.AssigneeID == nil || *gotChild.AssigneeID != preservedAssignee.ID {
		t.Fatalf("child assignee = %v, want preserved assignee %q", gotChild.AssigneeID, preservedAssignee.ID)
	}

	gotGrandchild, err := store.Tasks.Get(ctx, grandchild.ID)
	if err != nil {
		t.Fatalf("Get grandchild: %v", err)
	}
	if gotGrandchild.ProjectID != projTarget.ID {
		t.Fatalf("grandchild project_id = %q, want %q", gotGrandchild.ProjectID, projTarget.ID)
	}
	if gotGrandchild.ParentID == nil || *gotGrandchild.ParentID != child.ID {
		t.Fatalf("grandchild parent_id = %v, want %q", gotGrandchild.ParentID, child.ID)
	}
	if gotGrandchild.Status != "todo" {
		t.Fatalf("grandchild status = %q, want todo fallback", gotGrandchild.Status)
	}
	if gotGrandchild.AssigneeID == nil || *gotGrandchild.AssigneeID != targetOwner.ID {
		t.Fatalf("grandchild assignee = %v, want target owner %q", gotGrandchild.AssigneeID, targetOwner.ID)
	}

	targetTopLevel, err := store.Tasks.ListChildren(ctx, projTarget.ID, nil, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren target top level: %v", err)
	}
	if len(targetTopLevel) != 2 {
		t.Fatalf("target top-level task count = %d, want 2", len(targetTopLevel))
	}
	if targetTopLevel[0].ID != targetExisting.ID || targetTopLevel[0].Position != 0 {
		t.Fatalf("target first task = (%q,%d), want (%q,0)", targetTopLevel[0].ID, targetTopLevel[0].Position, targetExisting.ID)
	}
	if targetTopLevel[1].ID != movedRoot.ID || targetTopLevel[1].Position != 1 {
		t.Fatalf("target second task = (%q,%d), want (%q,1)", targetTopLevel[1].ID, targetTopLevel[1].Position, movedRoot.ID)
	}

	remainingSourceChildren, err := store.Tasks.ListChildren(ctx, projSource.ID, &oldParent.ID, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren source old parent: %v", err)
	}
	if len(remainingSourceChildren) != 1 {
		t.Fatalf("remaining source child count = %d, want 1", len(remainingSourceChildren))
	}
	if remainingSourceChildren[0].ID != sourceSibling.ID || remainingSourceChildren[0].Position != 0 {
		t.Fatalf("remaining source sibling = (%q,%d), want (%q,0)", remainingSourceChildren[0].ID, remainingSourceChildren[0].Position, sourceSibling.ID)
	}
}

// ---- Recurrence ----

func TestTasks_Create_WithRecurrence(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	rec := "FREQ=WEEKLY"
	task := &model.Task{
		ProjectID:  proj.ID,
		Name:       "Weekly Task",
		OwnerID:    owner.ID,
		Status:     "todo",
		Recurrence: &rec,
	}
	if err := store.Tasks.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Recurrence == nil || *got.Recurrence != rec {
		t.Errorf("Recurrence = %v, want %q", got.Recurrence, rec)
	}
}

func TestTasks_Update_RecurringToDone_SpawnsNext(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	due := "2026-04-14"
	rec := "FREQ=DAILY"
	task := &model.Task{
		ProjectID:  proj.ID,
		Name:       "Daily Task",
		OwnerID:    owner.ID,
		Status:     "todo",
		DueDate:    &due,
		Recurrence: &rec,
	}
	if err := store.Tasks.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Tags.Add(ctx, task.ID, "urgent"); err != nil {
		t.Fatalf("Add tag: %v", err)
	}

	task.Status = "done"
	_, nextID, err := store.Tasks.Update(ctx, task)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if task.Status != "done" {
		t.Errorf("updated.Status = %q, want done", task.Status)
	}

	if nextID == nil {
		t.Fatal("next occurrence ID is nil, want a new occurrence ID")
	}

	// Verify the spawned task exists and has the expected properties
	next, err := store.Tasks.Get(ctx, *nextID)
	if err != nil {
		t.Fatalf("Get next occurrence: %v", err)
	}
	if next.DueDate == nil || *next.DueDate != "2026-04-15" {
		t.Errorf("next.DueDate = %v, want 2026-04-15", next.DueDate)
	}
	if next.Recurrence == nil || *next.Recurrence != rec {
		t.Errorf("next.Recurrence = %v, want %q", next.Recurrence, rec)
	}
	if next.Status != "todo" {
		t.Errorf("next.Status = %q, want todo (first project status)", next.Status)
	}
	if next.Name != task.Name {
		t.Errorf("next.Name = %q, want %q", next.Name, task.Name)
	}

	tags, err := store.Tags.ListForTask(ctx, next.ID)
	if err != nil {
		t.Fatalf("ListForTask next: %v", err)
	}
	if len(tags) != 1 || tags[0] != "urgent" {
		t.Errorf("next tags = %v, want [urgent]", tags)
	}
}

func TestTasks_Update_RecurringToDone_NoDueDate_ReturnsConflict(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	rec := "FREQ=DAILY"
	task := &model.Task{
		ProjectID:  proj.ID,
		Name:       "Daily Task",
		OwnerID:    owner.ID,
		Status:     "todo",
		Recurrence: &rec,
	}
	if err := store.Tasks.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	task.Status = "done"
	_, _, err := store.Tasks.Update(ctx, task)
	if err != repo.ErrConflict {
		t.Errorf("err = %v, want repo.ErrConflict (recurring task requires due_date)", err)
	}

	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get after failed update: %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("status = %q after failed update, want todo (unchanged)", got.Status)
	}
}

func TestTasks_Update_NonRecurringToDone_NoNext(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})
	task := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	task.Status = "done"
	_, nextID, err := store.Tasks.Update(ctx, task)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if task.Status != "done" {
		t.Errorf("updated.Status = %q, want done", task.Status)
	}
	if nextID != nil {
		t.Errorf("nextID = %v, want nil for non-recurring task", nextID)
	}
}

// ---- Cycle guard ----

func TestTasks_CycleGuard(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	taskA := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	taskB := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, ParentID: &taskA.ID}) // B is child of A

	t.Run("self-reference returns ErrConflict", func(t *testing.T) {
		orig := taskA.ParentID
		taskA.ParentID = &taskA.ID
		_, _, err := store.Tasks.Update(ctx, taskA)
		if err != repo.ErrConflict {
			t.Errorf("self-ref: err = %v, want repo.ErrConflict", err)
		}
		taskA.ParentID = orig
	})

	t.Run("descendant reference returns ErrConflict", func(t *testing.T) {
		orig := taskA.ParentID
		taskA.ParentID = &taskB.ID
		_, _, err := store.Tasks.Update(ctx, taskA)
		if err != repo.ErrConflict {
			t.Errorf("descendant-ref: err = %v, want repo.ErrConflict", err)
		}
		taskA.ParentID = orig
	})

	t.Run("parent unchanged after failed move", func(t *testing.T) {
		got, err := store.Tasks.Get(ctx, taskA.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.ParentID != nil {
			t.Errorf("parentID = %v, want nil (unchanged)", got.ParentID)
		}
	})
}

// ---- Position compaction with non-contiguous positions ----

// setPositions is a test helper that directly sets task positions via SQL.
func setPositions(t *testing.T, db *sql.DB, positions map[string]int) {
	t.Helper()
	for id, pos := range positions {
		if _, err := db.ExecContext(context.Background(),
			`UPDATE tasks SET position = $1 WHERE id = $2`, pos, id,
		); err != nil {
			t.Fatalf("set position for %s: %v", id, err)
		}
	}
}

func assertPositions(t *testing.T, store *postgres.Store, projectID string, parentID *string, expected map[string]int) {
	t.Helper()
	tasks, err := store.Tasks.ListChildren(context.Background(), projectID, parentID, repo.TaskFilter{})
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	for _, tk := range tasks {
		want, ok := expected[tk.ID]
		if !ok {
			continue
		}
		if tk.Position != want {
			t.Errorf("task %s position = %d, want %d", tk.ID[:8], tk.Position, want)
		}
	}
}

func TestTasks_Update_CompactionPreservesRelativeOrder(t *testing.T) {
	db, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	// Frontend sent non-contiguous positions: [0, 500, 999]
	setPositions(t, db, map[string]int{t0.ID: 0, t1.ID: 500, t2.ID: 999})

	// Name-only update triggers compaction without any reorder.
	// Relative order must be preserved: 0→0, 500→1, 999→2.
	t0.Name = "Renamed"
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t0.ID: 0,
		t1.ID: 1,
		t2.ID: 2,
	})
}

func TestTasks_Update_NonContiguousReorderUp(t *testing.T) {
	db, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	// Frontend sent non-contiguous positions: [0, 500, 999]
	setPositions(t, db, map[string]int{t0.ID: 0, t1.ID: 500, t2.ID: 999})

	// Move t2 (relative index 2) up to index 0
	t2.Position = 0
	if _, _, err := store.Tasks.Update(ctx, t2); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t2.ID: 0,
		t0.ID: 1,
		t1.ID: 2,
	})
}

func TestTasks_Update_NonContiguousReorderDown(t *testing.T) {
	db, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	// Frontend sent non-contiguous positions: [0, 500, 999]
	setPositions(t, db, map[string]int{t0.ID: 0, t1.ID: 500, t2.ID: 999})

	// Move t0 (relative index 0) down to index 2
	t0.Position = 2
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t1.ID: 0,
		t2.ID: 1,
		t0.ID: 2,
	})
}

func TestTasks_Update_PositionClampedTooLarge(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	// Request position 999 for a 3-task list — should be clamped to 2 (last index)
	t0.Position = 999
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t1.ID: 0,
		t2.ID: 1,
		t0.ID: 2,
	})
}

func TestTasks_Update_PositionClampedNegative(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	// Request position -5 — should be clamped to 0
	t1.Position = -5
	if _, _, err := store.Tasks.Update(ctx, t1); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t1.ID: 0,
		t0.ID: 1,
	})
}

func TestTasks_Move_NonContiguousPositions(t *testing.T) {
	db, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	projX := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})
	projY := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	task1 := seed.Task(t, store, seed.TaskInput{ProjectID: projX.ID, OwnerID: owner.ID})
	task2 := seed.Task(t, store, seed.TaskInput{ProjectID: projX.ID, OwnerID: owner.ID})
	task3 := seed.Task(t, store, seed.TaskInput{ProjectID: projY.ID, OwnerID: owner.ID})

	// Set non-contiguous positions in both projects
	setPositions(t, db, map[string]int{task1.ID: 0, task2.ID: 500, task3.ID: 9999})

	// Move task1 from projX to projY at position 0
	task1.ProjectID = projY.ID
	task1.ParentID = nil
	task1.Position = 0
	if _, _, err := store.Tasks.Update(ctx, task1); err != nil {
		t.Fatalf("Update (cross-project move): %v", err)
	}

	// projX: task2 should be compacted to position 0
	assertPositions(t, store, projX.ID, nil, map[string]int{
		task2.ID: 0,
	})

	// projY: task1 at 0, task3 compacted after it
	assertPositions(t, store, projY.ID, nil, map[string]int{
		task1.ID: 0,
		task3.ID: 1,
	})
}

func TestTasks_Update_SamePositionNoChange(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID})

	// Update t0 keeping same position — no siblings should move
	t0.Position = 0
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t0.ID: 0,
		t1.ID: 1,
	})
}

// ---- Status change with explicit position ----

func TestTasks_Update_StatusChangeWithPosition_InsertsAtPosition(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// Create tasks in "in_progress": ip0(pos0), ip1(pos1), ip2(pos2)
	ip0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "in_progress"})
	ip1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "in_progress"})
	ip2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "in_progress"})

	// Create tasks in "todo": td0(pos0), td1(pos1), td2(pos2)
	td0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	td1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	td2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move ip0 from in_progress to todo at position 1
	// todo should become: td0=0, ip0=1, td1=2, td2=3
	ip0.Status = "todo"
	ip0.Position = 1
	if _, _, err := store.Tasks.Update(ctx, ip0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify in_progress: ip1 and ip2 compacted
	ipStatus := "in_progress"
	ipTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &ipStatus})
	if err != nil {
		t.Fatalf("ListChildren in_progress: %v", err)
	}
	if len(ipTasks) != 2 {
		t.Fatalf("in_progress count = %d, want 2", len(ipTasks))
	}
	posMap := map[string]int{}
	for _, tk := range ipTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[ip1.ID] != 0 {
		t.Errorf("ip1 position = %d, want 0 (compacted)", posMap[ip1.ID])
	}
	if posMap[ip2.ID] != 1 {
		t.Errorf("ip2 position = %d, want 1 (compacted)", posMap[ip2.ID])
	}

	// Verify todo: td0=0, ip0=1, td1=2, td2=3
	todoStatus := "todo"
	todoTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &todoStatus})
	if err != nil {
		t.Fatalf("ListChildren todo: %v", err)
	}
	posMap = map[string]int{}
	for _, tk := range todoTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[td0.ID] != 0 {
		t.Errorf("td0 position = %d, want 0", posMap[td0.ID])
	}
	if posMap[ip0.ID] != 1 {
		t.Errorf("ip0 position in todo = %d, want 1", posMap[ip0.ID])
	}
	if posMap[td1.ID] != 2 {
		t.Errorf("td1 position = %d, want 2 (shifted)", posMap[td1.ID])
	}
	if posMap[td2.ID] != 3 {
		t.Errorf("td2 position = %d, want 3 (shifted)", posMap[td2.ID])
	}
}

func TestTasks_Update_StatusChangeWithPosition0_InsertsAtBeginning(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// Create tasks in todo: t0(pos0), t1(pos1), t2(pos2)
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Create one in_progress task
	ip0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "in_progress"})

	// Move ip0 from in_progress to todo at position 0
	// todo should become: [ip0_at_0, t0_at_1, t1_at_2, t2_at_3]
	ip0.Status = "todo"
	ip0.Position = 0
	if _, _, err := store.Tasks.Update(ctx, ip0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	todoStatus := "todo"
	todoTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &todoStatus})
	if err != nil {
		t.Fatalf("ListChildren todo: %v", err)
	}

	posMap := map[string]int{}
	for _, tk := range todoTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[ip0.ID] != 0 {
		t.Errorf("ip0 position = %d, want 0", posMap[ip0.ID])
	}
	if posMap[t0.ID] != 1 {
		t.Errorf("t0 position = %d, want 1 (shifted)", posMap[t0.ID])
	}
	if posMap[t1.ID] != 2 {
		t.Errorf("t1 position = %d, want 2 (shifted)", posMap[t1.ID])
	}
	if posMap[t2.ID] != 3 {
		t.Errorf("t2 position = %d, want 3 (shifted)", posMap[t2.ID])
	}
}

// ---- Status change without position (append to end) ----

func TestTasks_Update_StatusChangeWithoutPosition_AppendsToEnd(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// Create 3 tasks in todo
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Create 2 tasks in in_progress
	ip0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "in_progress"})
	_ = seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "in_progress"})

	// Move ip0 from in_progress to todo WITHOUT specifying position
	// Should append at the end of todo (position 3, after t0=0, t1=1, t2=2)
	// The handler sets position to len(siblings) when no position is given
	siblings, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: ptrStr("todo")})
	if err != nil {
		t.Fatalf("ListChildren todo: %v", err)
	}
	ip0.Status = "todo"
	ip0.Position = len(siblings) // handler does this when no position specified
	if _, _, err := store.Tasks.Update(ctx, ip0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify todo: t0=0, t1=1, t2=2, ip0=3
	todoStatus := "todo"
	todoTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &todoStatus})
	if err != nil {
		t.Fatalf("ListChildren todo: %v", err)
	}
	posMap := map[string]int{}
	for _, tk := range todoTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[ip0.ID] != 3 {
		t.Errorf("ip0 position in todo = %d, want 3 (appended)", posMap[ip0.ID])
	}
	if posMap[t0.ID] != 0 {
		t.Errorf("t0 position = %d, want 0", posMap[t0.ID])
	}
	if posMap[t1.ID] != 1 {
		t.Errorf("t1 position = %d, want 1", posMap[t1.ID])
	}
	if posMap[t2.ID] != 2 {
		t.Errorf("t2 position = %d, want 2", posMap[t2.ID])
	}

	// Verify in_progress: ip1 compacted to 0
	ipStatus := "in_progress"
	ipTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &ipStatus})
	if err != nil {
		t.Fatalf("ListChildren in_progress: %v", err)
	}
	if len(ipTasks) != 1 || ipTasks[0].Position != 0 {
		t.Errorf("in_progress tasks = %v, want single task at position 0", ipTasks)
	}
}

// ---- Position change only (same status) ----

func TestTasks_Update_PositionOnly_InsertsAtPosition(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// Create 4 tasks in todo: t0=0, t1=1, t2=2, t3=3
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t3 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move t3 (pos=3) to position 2
	// Expected: t0=0, t1=1, t3=2, t2=3
	t3.Position = 2
	if _, _, err := store.Tasks.Update(ctx, t3); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertPositions(t, store, proj.ID, nil, map[string]int{
		t0.ID: 0,
		t1.ID: 1,
		t3.ID: 2,
		t2.ID: 3,
	})
}

// ---- Status change to empty group with position ----

func TestTasks_Update_StatusChangeToEmptyGroup_WithPosition(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// Create task in todo
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	_ = seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move to in_progress (empty group) at position 1
	// Position 1 in empty group should be clamped to 0
	t0.Status = "in_progress"
	t0.Position = 1
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	ipStatus := "in_progress"
	ipTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &ipStatus})
	if err != nil {
		t.Fatalf("ListChildren in_progress: %v", err)
	}
	if len(ipTasks) != 1 || ipTasks[0].Position != 0 {
		t.Errorf("in_progress task position = %d, want 0 (clamped from 1)", ipTasks[0].Position)
	}
}

// ---- Status change to empty group without position ----

func TestTasks_Update_StatusChangeToEmptyGroup_WithoutPosition(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	_ = seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move to in_progress (empty group), handler sets position = 0 (len of empty siblings)
	siblings, _ := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: ptrStr("in_progress")})
	t0.Status = "in_progress"
	t0.Position = len(siblings)
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	ipStatus := "in_progress"
	ipTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &ipStatus})
	if err != nil {
		t.Fatalf("ListChildren in_progress: %v", err)
	}
	if len(ipTasks) != 1 || ipTasks[0].Position != 0 {
		t.Errorf("in_progress task position = %d, want 0", ipTasks[0].Position)
	}
}

// ---- Compaction of source group after status change ----

func TestTasks_Update_StatusChange_CompactsSourceGroup(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// 3 tasks in todo
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})
	t2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move t1 from todo to in_progress at position 0
	t1.Status = "in_progress"
	t1.Position = 0
	if _, _, err := store.Tasks.Update(ctx, t1); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Source group (todo) should be compacted: t0=0, t2=1
	todoStatus := "todo"
	todoTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &todoStatus})
	if err != nil {
		t.Fatalf("ListChildren todo: %v", err)
	}
	posMap := map[string]int{}
	for _, tk := range todoTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[t0.ID] != 0 {
		t.Errorf("t0 position = %d, want 0", posMap[t0.ID])
	}
	if posMap[t2.ID] != 1 {
		t.Errorf("t2 position = %d, want 1 (compacted from 2)", posMap[t2.ID])
	}
}

// ---- Status change with position in the middle of a populated group ----

func TestTasks_Update_StatusChangeWithPosition_InsertsInMiddle(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	// 3 tasks in done
	d0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "done"})
	d1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "done"})
	d2 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "done"})

	// 1 task in todo
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move t0 from todo to done at position 1
	// done should become: d0=0, t0=1, d1=2, d2=3
	t0.Status = "done"
	t0.Position = 1
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	doneStatus := "done"
	doneTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &doneStatus})
	if err != nil {
		t.Fatalf("ListChildren done: %v", err)
	}
	posMap := map[string]int{}
	for _, tk := range doneTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[d0.ID] != 0 {
		t.Errorf("d0 position = %d, want 0", posMap[d0.ID])
	}
	if posMap[t0.ID] != 1 {
		t.Errorf("t0 position = %d, want 1 (inserted)", posMap[t0.ID])
	}
	if posMap[d1.ID] != 2 {
		t.Errorf("d1 position = %d, want 2 (shifted)", posMap[d1.ID])
	}
	if posMap[d2.ID] != 3 {
		t.Errorf("d2 position = %d, want 3 (shifted)", posMap[d2.ID])
	}
}

func ptrStr(s string) *string { return &s }

func TestTasks_Update_StatusChange_PositionClampedTooLarge(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	d0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "done"})
	d1 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "done"})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move t0 to done at position 999 — should clamp to 2 (siblingCount=2, joining)
	t0.Status = "done"
	t0.Position = 999
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	doneStatus := "done"
	doneTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &doneStatus})
	if err != nil {
		t.Fatalf("ListChildren done: %v", err)
	}
	posMap := map[string]int{}
	for _, tk := range doneTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[d0.ID] != 0 {
		t.Errorf("d0 position = %d, want 0", posMap[d0.ID])
	}
	if posMap[d1.ID] != 1 {
		t.Errorf("d1 position = %d, want 1", posMap[d1.ID])
	}
	if posMap[t0.ID] != 2 {
		t.Errorf("t0 position = %d, want 2 (clamped append)", posMap[t0.ID])
	}
}

func TestTasks_Update_StatusChangeToEmptyGroup_PositionClampedTooLarge(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move to done (empty) at position 999 — should clamp to 0
	t0.Status = "done"
	t0.Position = 999
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	doneStatus := "done"
	doneTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &doneStatus})
	if err != nil {
		t.Fatalf("ListChildren done: %v", err)
	}
	if len(doneTasks) != 1 || doneTasks[0].Position != 0 {
		t.Errorf("done task position = %d, want 0 (clamped from 999 in empty group)", doneTasks[0].Position)
	}
}

func TestTasks_Update_StatusChange_PositionNegative(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@test.com"})
	proj := seed.Project(t, store, seed.ProjectInput{OwnerID: owner.ID})

	d0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "done"})
	t0 := seed.Task(t, store, seed.TaskInput{ProjectID: proj.ID, OwnerID: owner.ID, Status: "todo"})

	// Move t0 to done at position -5 — should clamp to 0
	t0.Status = "done"
	t0.Position = -5
	if _, _, err := store.Tasks.Update(ctx, t0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	doneStatus := "done"
	doneTasks, err := store.Tasks.ListChildren(ctx, proj.ID, nil, repo.TaskFilter{Status: &doneStatus})
	if err != nil {
		t.Fatalf("ListChildren done: %v", err)
	}
	posMap := map[string]int{}
	for _, tk := range doneTasks {
		posMap[tk.ID] = tk.Position
	}
	if posMap[t0.ID] != 0 {
		t.Errorf("t0 position = %d, want 0 (clamped from -5)", posMap[t0.ID])
	}
	if posMap[d0.ID] != 1 {
		t.Errorf("d0 position = %d, want 1 (shifted)", posMap[d0.ID])
	}
}
