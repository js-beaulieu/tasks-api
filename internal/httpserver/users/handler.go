package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	"github.com/js-beaulieu/tasks/internal/repo"
)

type Handler struct {
	users repo.UserRepo
}

// NewRouter returns a chi router for user routes.
// Mount at /users in the main server.
func NewRouter(users repo.UserRepo) http.Handler {
	h := &Handler{users: users}
	r := chi.NewRouter()
	r.Get("/me", h.GetMe)
	return r
}

// GET /users/me — returns the authenticated user from context.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r.Context())
	render.JSON(w, http.StatusOK, u)
}
