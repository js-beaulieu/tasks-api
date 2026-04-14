//go:build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

const testDSN = "file::memory:?cache=shared&_pragma=foreign_keys(ON)"

func TestUsers_GetOrCreate(t *testing.T) {
	db, err := sqlite.Open(testDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	store := sqlite.New(db)
	ctx := context.Background()

	t.Run("creates a new user", func(t *testing.T) {
		u, err := store.Users.GetOrCreate(ctx, "user-1", "Alice", "alice@example.com")
		if err != nil {
			t.Fatalf("GetOrCreate: %v", err)
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

	t.Run("idempotent on same ID", func(t *testing.T) {
		first, err := store.Users.GetOrCreate(ctx, "user-2", "Bob", "bob@example.com")
		if err != nil {
			t.Fatalf("first GetOrCreate: %v", err)
		}
		second, err := store.Users.GetOrCreate(ctx, "user-2", "Bobby", "bobby@example.com")
		if err != nil {
			t.Fatalf("second GetOrCreate: %v", err)
		}
		if first.ID != second.ID {
			t.Errorf("IDs differ: %q vs %q", first.ID, second.ID)
		}
		if second.Name != "Bob" {
			t.Errorf("Name changed to %q, want original %q", second.Name, "Bob")
		}
	})
}

func TestUsers_GetByID(t *testing.T) {
	db, err := sqlite.Open(testDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	store := sqlite.New(db)
	ctx := context.Background()

	t.Run("existing user", func(t *testing.T) {
		_, err := store.Users.GetOrCreate(ctx, "user-3", "Carol", "carol@example.com")
		if err != nil {
			t.Fatalf("GetOrCreate: %v", err)
		}
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
