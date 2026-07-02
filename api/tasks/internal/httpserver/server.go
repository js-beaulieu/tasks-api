package httpserver

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/config"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/projects"
	taghandler "github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/tags"
	taskhandler "github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/tasks"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/users"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/store/postgres"
	"github.com/js-beaulieu/hs-api/libs/hs-common/render"
)

func New(store *postgres.Store, cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	apiConfig := huma.DefaultConfig("hs-api-tasks", "1.0.0")
	apiConfig.OpenAPIPath = "/openapi"
	apiConfig.DocsPath = "/docs"
	huma.DefaultArrayNullable = false
	if cfg.OpenAPIServerURL != "" {
		apiConfig.Servers = []*huma.Server{{URL: cfg.OpenAPIServerURL}}
	}
	api := humago.New(mux, apiConfig)

	protected := huma.NewGroup(api)
	protected.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		req, w := humago.Unwrap(ctx)
		middleware.AuthMiddleware(store.Users)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			next(huma.WithContext(ctx, r.Context()))
		})).ServeHTTP(w, req)
	})

	users.RegisterRoutes(protected, store.Users, "/users")
	projects.RegisterRoutes(protected, store.Projects, store.Tasks, "/projects")
	taskhandler.RegisterRoutes(protected, store.Projects, store.Tasks, store.Tags, "/tasks")
	taghandler.RegisterRoutes(protected, store.Tags, "/tags")

	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
