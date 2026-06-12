package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks-api/internal/config"
	httpmdw "github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/tags"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/users"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

type apiError struct {
	Error string `json:"error"`
}

func init() {
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		return &statusErr{status: status, msg: msg}
	}
}

type statusErr struct {
	status int
	msg    string
}

func (e *statusErr) Error() string {
	return e.msg
}

func (e *statusErr) GetStatus() int {
	return e.status
}

func (e *statusErr) ContentType(string) string {
	return "application/json"
}

func (e *statusErr) MarshalJSON() ([]byte, error) {
	return json.Marshal(apiError{Error: e.msg})
}

func New(store *postgres.Store, cfg config.Config) http.Handler {
	_ = cfg
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Group(func(r chi.Router) {
		r.Use(httpmdw.AuthMiddleware(store.Users))
		api := humachi.New(r, render.HumaConfig())
		users.Register(api, store.Users)
		projects.Register(api, store.Projects, store.Tasks)
		tasks.Register(api, store.Projects, store.Tasks, store.Tags)
		tags.Register(api, store.Tags)
	})

	return r
}
