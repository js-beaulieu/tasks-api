package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
	r.Get("/me", h.getMe)
	r.Patch("/me", h.updateMe)
	r.Get("/{userID}", h.getByID)
	return r
}

// GET /users/me — returns the authenticated user from context.
func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, middleware.UserFromCtx(r.Context()))
}

// GET /users/{userID} — returns a single user by ID.
func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.GetByID(r.Context(), chi.URLParam(r, "userID"))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			render.NotFound(w)
			return
		}
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, u)
}

type updateMeReq struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

// PATCH /users/me — updates the authenticated user's name and/or email.
func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var body updateMeReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	u := middleware.UserFromCtx(r.Context())
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			render.BadRequest(w, "name cannot be blank")
			return
		}
		u.Name = *body.Name
	}
	if body.Email != nil {
		if strings.TrimSpace(*body.Email) == "" {
			render.BadRequest(w, "email cannot be blank")
			return
		}
		u.Email = *body.Email
	}
	if err := h.users.Update(r.Context(), u); err != nil {
		if errors.Is(err, repo.ErrConflict) {
			render.Error(w, http.StatusConflict, "email already in use")
			return
		}
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, u)
}

// LoginHandler returns an http.HandlerFunc for POST /login.
// It explicitly creates a user on first login and returns the user record.
// Must be mounted outside the auth middleware since the user may not exist yet.
func LoginHandler(users repo.UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-User-ID")
		if id == "" {
			render.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		name := r.Header.Get("X-User-Name")
		email := r.Header.Get("X-User-Email")

		u, err := users.Create(r.Context(), id, name, email)
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				u, err = users.GetByID(r.Context(), id)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "internal error")
					return
				}
			} else {
				render.Error(w, http.StatusInternalServerError, "internal error")
				return
			}
		}
		render.JSON(w, http.StatusOK, u)
	}
}
