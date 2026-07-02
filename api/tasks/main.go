package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/config"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver"
	httpmdw "github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/mcpserver"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/store/postgres"
	"github.com/js-beaulieu/hs-api/libs/hs-common/logger"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger.New(logger.Options{
		Format: cfg.LogFormat,
		Level:  cfg.LogLevel,
	})
	slog.Info("starting server", "port", cfg.Port)

	db, err := postgres.Open(cfg.PGConnectionString)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	store := postgres.New(db)

	mux := http.NewServeMux()
	mux.Handle("/", httpserver.New(store, cfg))
	mux.Handle("/mcp", httpmdw.AuthMiddleware(store.Users)(mcpserver.Handler(store, cfg)))
	h := httpmdw.Logging(cfg)(mux)

	slog.Info("listening", "addr", ":"+cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, h); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
