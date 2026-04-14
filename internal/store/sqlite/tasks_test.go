//go:build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	testdb "github.com/js-beaulieu/tasks/internal/testing/db"
	"github.com/js-beaulieu/tasks/internal/testing/seed"
)

// ---- Create / Get ----

func TestTasks_CreateGet(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	t1 := seed.Task(t, store, proj.ID, owner.ID, nil)
	t2 := seed.Task(t, store, proj.ID, owner.ID, nil)

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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	t1 := seed.Task(t, store, proj.ID, owner.ID, nil)
	t2 := seed.Task(t, store, proj.ID, owner.ID, nil)

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
		if err := store.Tasks.Update(ctx, t2); err != nil {
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
		assignee := seed.User(t, store, "u2", "Bob", "bob@test.com")
		t1.AssigneeID = &assignee.ID
		if err := store.Tasks.Update(ctx, t1); err != nil {
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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	originalUpdatedAt := task.UpdatedAt
	task.Name = "Updated Name"
	if err := store.Tasks.Update(ctx, task); err != nil {
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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	task.Status = "bogus"
	err := store.Tasks.Update(ctx, task)
	if err != repo.ErrConflict {
		t.Errorf("err = %v, want repo.ErrConflict for invalid status", err)
	}
}

func TestTasks_Update_PositionReorderUp(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	t0 := seed.Task(t, store, proj.ID, owner.ID, nil) // pos 0
	t1 := seed.Task(t, store, proj.ID, owner.ID, nil) // pos 1
	t2 := seed.Task(t, store, proj.ID, owner.ID, nil) // pos 2

	// Move t2 (pos=2) up to pos=0
	t2.Position = 0
	if err := store.Tasks.Update(ctx, t2); err != nil {
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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	t0 := seed.Task(t, store, proj.ID, owner.ID, nil) // pos 0
	t1 := seed.Task(t, store, proj.ID, owner.ID, nil) // pos 1
	t2 := seed.Task(t, store, proj.ID, owner.ID, nil) // pos 2

	// Move t0 (pos=0) down to pos=2
	t0.Position = 2
	if err := store.Tasks.Update(ctx, t0); err != nil {
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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	parent := seed.Task(t, store, proj.ID, owner.ID, nil)
	child := seed.Task(t, store, proj.ID, owner.ID, &parent.ID)

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

// ---- Move between parents (same project) ----

func TestTasks_Move_BetweenParents(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	parentA := seed.Task(t, store, proj.ID, owner.ID, nil)
	parentB := seed.Task(t, store, proj.ID, owner.ID, nil)
	child := seed.Task(t, store, proj.ID, owner.ID, &parentA.ID)

	// Move child from parentA to parentB
	child.ParentID = &parentB.ID
	child.Position = 0
	if err := store.Tasks.Update(ctx, child); err != nil {
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
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	projX := seed.Project(t, store, owner.ID)
	projY := seed.Project(t, store, owner.ID)

	task := seed.Task(t, store, projX.ID, owner.ID, nil)
	// Extra task in projX to verify position compaction
	_ = seed.Task(t, store, projX.ID, owner.ID, nil)

	// Move task from projX to projY
	task.ProjectID = projY.ID
	task.ParentID = nil
	task.Position = 0
	if err := store.Tasks.Update(ctx, task); err != nil {
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

// ---- Recurrence ----

func TestTasks_Create_WithRecurrence(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

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

func TestTasks_CompleteTask_NonRecurring(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	completed, next, err := store.Tasks.CompleteTask(ctx, task.ID, "done")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if completed == nil {
		t.Fatal("completed task is nil")
	}
	if completed.Status != "done" {
		t.Errorf("completed.Status = %q, want done", completed.Status)
	}
	if next != nil {
		t.Errorf("next = %v, want nil for non-recurring task", next)
	}
}

func TestTasks_CompleteTask_RecurringNoDueDate(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	rec := "FREQ=DAILY"
	task := &model.Task{
		ProjectID:  proj.ID,
		Name:       "Daily Task",
		OwnerID:    owner.ID,
		Status:     "todo",
		Recurrence: &rec,
		// intentionally no DueDate
	}
	if err := store.Tasks.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, _, err := store.Tasks.CompleteTask(ctx, task.ID, "done")
	if err != repo.ErrConflict {
		t.Errorf("err = %v, want repo.ErrConflict (recurring task requires due_date)", err)
	}

	// status must remain unchanged
	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get after failed complete: %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("status = %q after failed complete, want todo (unchanged)", got.Status)
	}
}

func TestTasks_CompleteTask_Recurring(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

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

	completed, next, err := store.Tasks.CompleteTask(ctx, task.ID, "done")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	if completed == nil {
		t.Fatal("completed task is nil")
	}
	if completed.Status != "done" {
		t.Errorf("completed.Status = %q, want done", completed.Status)
	}

	if next == nil {
		t.Fatal("next task is nil, want a new occurrence")
	}
	if next.ID == task.ID {
		t.Error("next.ID must be a new UUID, not the same as the original")
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

	// tags must be copied to the next occurrence
	tags, err := store.Tags.ListForTask(ctx, next.ID)
	if err != nil {
		t.Fatalf("ListForTask next: %v", err)
	}
	if len(tags) != 1 || tags[0] != "urgent" {
		t.Errorf("next tags = %v, want [urgent]", tags)
	}
}

func TestTasks_CompleteTask_InvalidDoneStatus(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	_, _, err := store.Tasks.CompleteTask(ctx, task.ID, "nonexistent_status")
	if err != repo.ErrConflict {
		t.Errorf("err = %v, want repo.ErrConflict (invalid done_status)", err)
	}

	got, err := store.Tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("status = %q after failed complete, want todo (unchanged)", got.Status)
	}
}

// ---- Cycle guard ----

func TestTasks_CycleGuard(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)

	taskA := seed.Task(t, store, proj.ID, owner.ID, nil)
	taskB := seed.Task(t, store, proj.ID, owner.ID, &taskA.ID) // B is child of A

	t.Run("self-reference returns ErrConflict", func(t *testing.T) {
		orig := taskA.ParentID
		taskA.ParentID = &taskA.ID
		err := store.Tasks.Update(ctx, taskA)
		if err != repo.ErrConflict {
			t.Errorf("self-ref: err = %v, want repo.ErrConflict", err)
		}
		taskA.ParentID = orig
	})

	t.Run("descendant reference returns ErrConflict", func(t *testing.T) {
		orig := taskA.ParentID
		taskA.ParentID = &taskB.ID
		err := store.Tasks.Update(ctx, taskA)
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
