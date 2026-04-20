package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks/internal/config"
	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	taghandler "github.com/js-beaulieu/tasks/internal/httpserver/tags"
	taskhandler "github.com/js-beaulieu/tasks/internal/httpserver/tasks"
	"github.com/js-beaulieu/tasks/internal/httpserver/users"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func New(store *sqlite.Store, cfg config.Config) http.Handler {
	r := chi.NewRouter()

	r.Get("/health", healthHandler)
	r.Get("/.well-known/oauth-authorization-server", wellKnownHandler(cfg))

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(store.Users))
		r.Mount("/users", users.NewRouter(store.Users))
		r.Mount("/projects", projects.NewRouter(store.Projects, store.Tasks))
		r.Mount("/tasks", taskhandler.NewRouter(store.Projects, store.Tasks, store.Tags))
		r.Mount("/tags", taghandler.NewRouter(store.Tags))
	})

	return r
}

func wellKnownHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.ZitadelIssuer == "" {
			render.Error(w, http.StatusServiceUnavailable, "authorization server not configured")
			return
		}
		render.JSON(w, http.StatusOK, map[string]any{
			"issuer":                           cfg.ZitadelIssuer,
			"authorization_endpoint":           cfg.ZitadelAuthURL,
			"token_endpoint":                   cfg.ZitadelTokenURL,
			"jwks_uri":                         cfg.ZitadelJWKSURL,
			"response_types_supported":         []string{"code"},
			"grant_types_supported":            []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported": []string{"S256"},
		})
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
