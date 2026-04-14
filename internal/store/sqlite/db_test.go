package sqlite_test

import (
	"fmt"
	"testing"

	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func TestOpen_InMemory(t *testing.T) {
	t.Run("valid in-memory DSN succeeds", func(t *testing.T) {
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(ON)", t.Name())
		db, err := sqlite.Open(dsn)
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
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(ON)", t.Name())
	db, err := sqlite.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	store := sqlite.New(db)
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
