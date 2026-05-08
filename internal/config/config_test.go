package config

import (
	"testing"
)

func TestLoadDatabaseURL(t *testing.T) {
	t.Run("defaults to local postgres when DATABASE_URL unset", func(t *testing.T) {
		cfg := Load()
		want := "postgres://postgres:postgres@localhost:5432/tasks_api?sslmode=disable"
		if cfg.DatabaseURL != want {
			t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
		}
	})

	t.Run("reads DATABASE_URL from env", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://app:secret@db:5432/tasks_api?sslmode=disable")
		cfg := Load()
		want := "postgres://app:secret@db:5432/tasks_api?sslmode=disable"
		if cfg.DatabaseURL != want {
			t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
		}
	})
}
