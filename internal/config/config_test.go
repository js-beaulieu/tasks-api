package config

import (
	"testing"
)

func TestLoadDBPath(t *testing.T) {
	t.Run("defaults to tasks.db when DB_PATH unset", func(t *testing.T) {
		cfg := Load()
		if cfg.DBPath != "tasks.db" {
			t.Errorf("DBPath = %q, want tasks.db", cfg.DBPath)
		}
	})

	t.Run("reads DB_PATH from env", func(t *testing.T) {
		t.Setenv("DB_PATH", "/data/tasks.db")
		cfg := Load()
		if cfg.DBPath != "/data/tasks.db" {
			t.Errorf("DBPath = %q, want /data/tasks.db", cfg.DBPath)
		}
	})
}
