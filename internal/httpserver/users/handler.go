package users

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

type Handler struct {
	users repo.UserRepo
}

func Register(api huma.API, users repo.UserRepo) {
	h := &Handler{users: users}
	register(api, h, "/users")
}

func NewRouter(users repo.UserRepo) http.Handler {
	r := chi.NewRouter()
	api := humachi.New(r, render.HumaConfig())
	register(api, &Handler{users: users}, "")
	return r
}

func register(api huma.API, h *Handler, prefix string) {
	huma.Get(api, route(prefix, "/me"), h.getMe)
	huma.Patch(api, route(prefix, "/me"), h.updateMe)
	huma.Get(api, route(prefix, "/{userID}"), h.getByID)
}

func route(prefix, path string) string {
	if prefix == "" {
		return path
	}
	return prefix + path
}

type meOutput struct {
	Body model.User
}

func (h *Handler) getMe(ctx context.Context, _ *struct{}) (*meOutput, error) {
	u := middleware.UserFromCtx(ctx)
	return &meOutput{Body: *u}, nil
}

type getUserOutput struct {
	Body model.User
}

func (h *Handler) getByID(ctx context.Context, input *struct {
	UserID string `path:"userID"`
}) (*getUserOutput, error) {
	u, err := h.users.GetByID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, huma.Error404NotFound("not found")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &getUserOutput{Body: *u}, nil
}

type updateMeInput struct {
	Body struct {
		Name  *string `json:"name,omitempty"`
		Email *string `json:"email,omitempty"`
	}
}

type updateMeOutput struct {
	Body model.User
}

func (h *Handler) updateMe(ctx context.Context, input *updateMeInput) (*updateMeOutput, error) {
	u := middleware.UserFromCtx(ctx)
	if input.Body.Name != nil {
		if strings.TrimSpace(*input.Body.Name) == "" {
			return nil, huma.Error400BadRequest("name cannot be blank")
		}
		u.Name = *input.Body.Name
	}
	if input.Body.Email != nil {
		if strings.TrimSpace(*input.Body.Email) == "" {
			return nil, huma.Error400BadRequest("email cannot be blank")
		}
		u.Email = *input.Body.Email
	}
	if err := h.users.Update(ctx, u); err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return nil, huma.Error409Conflict("email already in use")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &updateMeOutput{Body: *u}, nil
}
