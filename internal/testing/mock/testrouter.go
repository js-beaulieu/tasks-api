package mock

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
)

// NewTestRouter creates a chi router with Huma registered via fn,
// wrapped in AuthMiddleware using the provided UserRepo.
func NewTestRouter(users *UserRepo, fn func(api huma.API)) http.Handler {
	r := chi.NewRouter()
	api := humachi.New(r, render.HumaConfig())
	fn(api)
	return middleware.AuthMiddleware(users)(r)
}
