package config

import (
	"testing"
)

func TestLoadPGConnectionString(t *testing.T) {
	t.Run("defaults to local postgres when PG_CONNECTION_STRING unset", func(t *testing.T) {
		cfg := Load()
		want := "postgres://postgres:postgres@localhost:5432/tasks_api?sslmode=disable"
		if cfg.PGConnectionString != want {
			t.Errorf("PGConnectionString = %q, want %q", cfg.PGConnectionString, want)
		}
	})

	t.Run("reads PG_CONNECTION_STRING from env", func(t *testing.T) {
		t.Setenv("PG_CONNECTION_STRING", "postgres://app:secret@db:5432/tasks_api?sslmode=disable")
		cfg := Load()
		want := "postgres://app:secret@db:5432/tasks_api?sslmode=disable"
		if cfg.PGConnectionString != want {
			t.Errorf("PGConnectionString = %q, want %q", cfg.PGConnectionString, want)
		}
	})
}
