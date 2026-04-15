//go:build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks/internal/repo"
	testdb "github.com/js-beaulieu/tasks/internal/testing/db"
	"github.com/js-beaulieu/tasks/internal/testing/seed"
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
