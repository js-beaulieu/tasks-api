//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
	testdb "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/db"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/seed"
)

func TestUsers_Create(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()

	t.Run("creates a new user", func(t *testing.T) {
		u, err := store.Users.Create(ctx, "user-1", "Alice", "alice@example.com")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if u.ID != "user-1" {
			t.Errorf("ID = %q, want %q", u.ID, "user-1")
		}
		if u.Name != "Alice" {
			t.Errorf("Name = %q, want %q", u.Name, "Alice")
		}
		if u.Email != "alice@example.com" {
			t.Errorf("Email = %q, want %q", u.Email, "alice@example.com")
		}
		if u.CreatedAt.IsZero() {
			t.Error("CreatedAt is zero")
		}
	})

	t.Run("duplicate ID returns ErrConflict", func(t *testing.T) {
		_, err := store.Users.Create(ctx, "user-2", "Bob", "bob@example.com")
		if err != nil {
			t.Fatalf("first Create: %v", err)
		}
		_, err = store.Users.Create(ctx, "user-2", "Bobby", "bobby@example.com")
		if err != repo.ErrConflict {
			t.Errorf("duplicate Create: err = %v, want repo.ErrConflict", err)
		}
	})
}

func TestUsers_GetByID(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()

	t.Run("existing user", func(t *testing.T) {
		seed.User(t, store, seed.UserInput{ID: "user-3", Name: "Carol", Email: "carol@example.com"})
		u, err := store.Users.GetByID(ctx, "user-3")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if u.ID != "user-3" {
			t.Errorf("ID = %q, want %q", u.ID, "user-3")
		}
		if u.Name != "Carol" {
			t.Errorf("Name = %q, want %q", u.Name, "Carol")
		}
	})

	t.Run("missing ID returns ErrNotFound", func(t *testing.T) {
		_, err := store.Users.GetByID(ctx, "does-not-exist")
		if err != repo.ErrNotFound {
			t.Errorf("err = %v, want repo.ErrNotFound", err)
		}
	})
}

func TestUsers_Update(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()

	t.Run("updates name and email", func(t *testing.T) {
		u := seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@example.com"})
		u.Name = "Alicia"
		u.Email = "alicia@example.com"
		if err := store.Users.Update(ctx, u); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := store.Users.GetByID(ctx, "u1")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Name != "Alicia" {
			t.Errorf("Name = %q, want %q", got.Name, "Alicia")
		}
		if got.Email != "alicia@example.com" {
			t.Errorf("Email = %q, want %q", got.Email, "alicia@example.com")
		}
	})

	t.Run("unknown ID returns ErrNotFound", func(t *testing.T) {
		u := seed.User(t, store, seed.UserInput{ID: "u2", Name: "Bob", Email: "bob@example.com"})
		u.ID = "does-not-exist"
		if err := store.Users.Update(ctx, u); err != repo.ErrNotFound {
			t.Errorf("err = %v, want repo.ErrNotFound", err)
		}
	})

	t.Run("duplicate email returns ErrConflict", func(t *testing.T) {
		seed.User(t, store, seed.UserInput{ID: "u3", Name: "Carol", Email: "carol@example.com"})
		u := seed.User(t, store, seed.UserInput{ID: "u4", Name: "Dave", Email: "dave@example.com"})
		u.Email = "carol@example.com"
		if err := store.Users.Update(ctx, u); err != repo.ErrConflict {
			t.Errorf("err = %v, want repo.ErrConflict", err)
		}
	})
}

func TestUsers_Delete(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()

	t.Run("deletes existing user", func(t *testing.T) {
		seed.User(t, store, seed.UserInput{ID: "u1", Name: "Alice", Email: "alice@example.com"})
		if err := store.Users.Delete(ctx, "u1"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := store.Users.GetByID(ctx, "u1"); err != repo.ErrNotFound {
			t.Errorf("after delete: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("unknown ID returns ErrNotFound", func(t *testing.T) {
		if err := store.Users.Delete(ctx, "does-not-exist"); err != repo.ErrNotFound {
			t.Errorf("err = %v, want repo.ErrNotFound", err)
		}
	})
}

func TestUsers_ListByIDs(t *testing.T) {
	_, store := testdb.Open(t)
	ctx := context.Background()

	t.Run("returns matching users", func(t *testing.T) {
		seed.User(t, store, seed.UserInput{ID: "lb-1", Name: "Alice", Email: "alice-lb@example.com"})
		seed.User(t, store, seed.UserInput{ID: "lb-2", Name: "Bob", Email: "bob-lb@example.com"})

		users, err := store.Users.ListByIDs(ctx, []string{"lb-1", "lb-2"})
		if err != nil {
			t.Fatalf("ListByIDs: %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("len = %d, want 2", len(users))
		}
	})

	t.Run("omits non-existent IDs", func(t *testing.T) {
		seed.User(t, store, seed.UserInput{ID: "lb-3", Name: "Carol", Email: "carol-lb@example.com"})

		users, err := store.Users.ListByIDs(ctx, []string{"lb-3", "nonexistent"})
		if err != nil {
			t.Fatalf("ListByIDs: %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("len = %d, want 1", len(users))
		}
		if users[0].ID != "lb-3" {
			t.Errorf("ID = %q, want %q", users[0].ID, "lb-3")
		}
	})

	t.Run("returns empty slice for no matches", func(t *testing.T) {
		users, err := store.Users.ListByIDs(ctx, []string{"no-match-1", "no-match-2"})
		if err != nil {
			t.Fatalf("ListByIDs: %v", err)
		}
		if len(users) != 0 {
			t.Errorf("len = %d, want 0", len(users))
		}
	})

	t.Run("returns nil for empty input", func(t *testing.T) {
		users, err := store.Users.ListByIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByIDs: %v", err)
		}
		if users != nil {
			t.Errorf("expected nil, got %v", users)
		}
	})
}
