package config

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Port            string
	DBPath          string
	LogFormat       string
	LogLevel        slog.Level
	LogDetailed     bool
	ZitadelIssuer   string
	ZitadelAuthURL  string
	ZitadelTokenURL string
	ZitadelJWKSURL  string
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
	if v := os.Getenv("ZITADEL_ISSUER"); v != "" {
		cfg.ZitadelIssuer = v
	}
	if v := os.Getenv("ZITADEL_AUTH_URL"); v != "" {
		cfg.ZitadelAuthURL = v
	}
	if v := os.Getenv("ZITADEL_TOKEN_URL"); v != "" {
		cfg.ZitadelTokenURL = v
	}
	if v := os.Getenv("ZITADEL_JWKS_URL"); v != "" {
		cfg.ZitadelJWKSURL = v
	}
	return cfg
}
