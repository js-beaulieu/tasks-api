package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	"github.com/js-beaulieu/tasks/internal/httpserver/users"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func New(store *sqlite.Store) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.AuthMiddleware(store.Users))
	r.Get("/health", healthHandler)
	r.Mount("/users", users.NewRouter(store.Users))
	return r
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
