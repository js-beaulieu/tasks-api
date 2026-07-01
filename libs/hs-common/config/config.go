// Package config provides lightweight environment-based configuration loading
// for Home Stack API services.
package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds common runtime settings for an API service.
type Config struct {
	Port               string
	PGConnectionString string
	OpenAPIServerURL   string
	LogFormat          string
	LogLevel           slog.Level
	LogDetailed        bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	cfg := Config{
		Port:               "8080",
		PGConnectionString: "postgres://postgres:postgres@localhost:5432/tasks_api?sslmode=disable",
		LogFormat:          "json",
		LogLevel:           slog.LevelInfo,
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("PG_CONNECTION_STRING"); v != "" {
		cfg.PGConnectionString = v
	}
	if v := os.Getenv("OPENAPI_SERVER_URL"); v != "" {
		cfg.OpenAPIServerURL = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		_ = cfg.LogLevel.UnmarshalText([]byte(v))
	}
	if v := os.Getenv("LOG_DETAILED"); v != "" {
		cfg.LogDetailed, _ = strconv.ParseBool(v)
	}
	return cfg
}
