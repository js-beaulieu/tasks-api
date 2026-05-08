//go:build integration

package postgres_test

import (
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
)

func TestOpen(t *testing.T) {
	t.Run("valid postgres dsn succeeds", func(t *testing.T) {
		db, err := postgres.Open(testdb.DatabaseURL(t))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			t.Fatalf("Ping after Open: %v", err)
		}
	})
}

func TestNew(t *testing.T) {
	db, err := postgres.Open(testdb.DatabaseURL(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	store := postgres.New(db)
	if store == nil {
		t.Fatal("New returned nil")
	}
	if store.Users == nil {
		t.Error("store.Users is nil")
	}
	if store.Projects == nil {
		t.Error("store.Projects is nil")
	}
	if store.Tasks == nil {
		t.Error("store.Tasks is nil")
	}
	if store.Tags == nil {
		t.Error("store.Tags is nil")
	}
}
