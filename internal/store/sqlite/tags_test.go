//go:build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
)

func TestTags_AddListForTask(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	if err := store.Tags.Add(ctx, task.ID, "backend"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	tags, err := store.Tags.ListForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListForTask: %v", err)
	}
	if len(tags) != 1 || tags[0] != "backend" {
		t.Errorf("tags = %v, want [backend]", tags)
	}
}

func TestTags_Add_Idempotent(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	if err := store.Tags.Add(ctx, task.ID, "dup"); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := store.Tags.Add(ctx, task.ID, "dup"); err != nil {
		t.Errorf("second Add (idempotent): %v", err)
	}

	tags, err := store.Tags.ListForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListForTask: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("len(tags) = %d, want 1 (no duplicates)", len(tags))
	}
}

func TestTags_Delete(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	proj := seed.Project(t, store, owner.ID)
	task := seed.Task(t, store, proj.ID, owner.ID, nil)

	if err := store.Tags.Add(ctx, task.ID, "remove-me"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := store.Tags.Delete(ctx, task.ID, "remove-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	tags, err := store.Tags.ListForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListForTask: %v", err)
	}
	for _, tag := range tags {
		if tag == "remove-me" {
			t.Error("deleted tag still present")
		}
	}
}

func TestTags_ListDistinctForUser(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()
	owner := seed.User(t, store, "u1", "Alice", "alice@test.com")
	outsider := seed.User(t, store, "u3", "Carol", "carol@test.com")

	// Owned project — tags here should be visible to owner
	ownedProj := seed.Project(t, store, owner.ID)
	t1 := seed.Task(t, store, ownedProj.ID, owner.ID, nil)
	if err := store.Tags.Add(ctx, t1.ID, "alpha"); err != nil {
		t.Fatalf("Add alpha: %v", err)
	}
	if err := store.Tags.Add(ctx, t1.ID, "beta"); err != nil {
		t.Fatalf("Add beta: %v", err)
	}

	// Shared project (outsider owns, owner is an explicit member)
	sharedProj := seed.Project(t, store, outsider.ID)
	if err := store.Projects.AddMember(ctx, &model.ProjectMember{
		ProjectID: sharedProj.ID,
		UserID:    owner.ID,
		Role:      model.RoleRead,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	t2 := seed.Task(t, store, sharedProj.ID, outsider.ID, nil)
	if err := store.Tags.Add(ctx, t2.ID, "gamma"); err != nil {
		t.Fatalf("Add gamma: %v", err)
	}

	// Project owner has no access to — tags here must NOT appear
	privateProj := seed.Project(t, store, outsider.ID)
	tPriv := seed.Task(t, store, privateProj.ID, outsider.ID, nil)
	if err := store.Tags.Add(ctx, tPriv.ID, "hidden"); err != nil {
		t.Fatalf("Add hidden: %v", err)
	}

	tags, err := store.Tags.ListDistinctForUser(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListDistinctForUser: %v", err)
	}

	found := map[string]bool{}
	for _, tag := range tags {
		found[tag] = true
	}
	if !found["alpha"] {
		t.Error("expected tag 'alpha' (owned project)")
	}
	if !found["beta"] {
		t.Error("expected tag 'beta' (owned project)")
	}
	if !found["gamma"] {
		t.Error("expected tag 'gamma' (shared project as member)")
	}
	if found["hidden"] {
		t.Error("tag 'hidden' from non-member project must not appear")
	}

	// Verify no duplicates and sorted
	seen := map[string]bool{}
	for i, tag := range tags {
		if seen[tag] {
			t.Errorf("duplicate tag %q", tag)
		}
		seen[tag] = true
		if i > 0 && tag < tags[i-1] {
			t.Errorf("tags not sorted: %q < %q", tag, tags[i-1])
		}
	}
}
