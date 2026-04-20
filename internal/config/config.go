package config

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DBPath      string
	LogFormat   string
	LogLevel    slog.Level
	LogDetailed bool
}

func Load() Config {
	cfg := Config{
		Port:      "8080",
		DBPath:    "tasks.db",
		LogFormat: "json",
		LogLevel:  slog.LevelInfo,
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
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
