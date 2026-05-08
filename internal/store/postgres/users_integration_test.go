//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/repo"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
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
		seed.User(t, store, "user-3", "Carol", "carol@example.com")
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
		u := seed.User(t, store, "u1", "Alice", "alice@example.com")
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
		u := seed.User(t, store, "u2", "Bob", "bob@example.com")
		u.ID = "does-not-exist"
		if err := store.Users.Update(ctx, u); err != repo.ErrNotFound {
			t.Errorf("err = %v, want repo.ErrNotFound", err)
		}
	})

	t.Run("duplicate email returns ErrConflict", func(t *testing.T) {
		seed.User(t, store, "u3", "Carol", "carol@example.com")
		u := seed.User(t, store, "u4", "Dave", "dave@example.com")
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
		seed.User(t, store, "u1", "Alice", "alice@example.com")
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
