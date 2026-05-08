package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"

	"github.com/js-beaulieu/tasks-api/internal/config"
	"github.com/js-beaulieu/tasks-api/internal/httpserver"
	httpmdw "github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/logger"
	"github.com/js-beaulieu/tasks-api/internal/mcpserver"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger.New(cfg)
	slog.Info("starting server", "port", cfg.Port)

	db, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	store := postgres.New(db)

	r := chi.NewRouter()
	r.Use(httpmdw.Logging(cfg))
	r.Mount("/", httpserver.New(store, cfg))
	r.Group(func(r chi.Router) {
		r.Use(httpmdw.AuthMiddleware(store.Users))
		r.Handle("/mcp", mcpserver.Handler(store, cfg))
	})

	slog.Info("listening", "addr", ":"+cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
