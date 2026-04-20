package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks-api/internal/config"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
	taghandler "github.com/js-beaulieu/tasks-api/internal/httpserver/tags"
	taskhandler "github.com/js-beaulieu/tasks-api/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/users"
	"github.com/js-beaulieu/tasks-api/internal/store/sqlite"
)

func New(store *sqlite.Store, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", healthHandler)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(store.Users))
		r.Mount("/users", users.NewRouter(store.Users))
		r.Mount("/projects", projects.NewRouter(store.Projects, store.Tasks))
		r.Mount("/tasks", taskhandler.NewRouter(store.Projects, store.Tasks, store.Tags))
		r.Mount("/tags", taghandler.NewRouter(store.Tags))
	})

	return r
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
