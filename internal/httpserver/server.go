package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	taghandler "github.com/js-beaulieu/tasks/internal/httpserver/tags"
	taskhandler "github.com/js-beaulieu/tasks/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks/internal/httpserver/users"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func New(store *sqlite.Store) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.AuthMiddleware(store.Users))
	r.Get("/health", healthHandler)
	r.Mount("/users", users.NewRouter(store.Users))
	r.Mount("/projects", projects.NewRouter(store.Projects, store.Tasks))
	r.Mount("/tasks", taskhandler.NewRouter(store.Projects, store.Tasks, store.Tags))
	r.Mount("/tags", taghandler.NewRouter(store.Tags))
	return r
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
