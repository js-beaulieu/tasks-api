package users

import (
	"context"
	"errors"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

type Handler struct {
	users repo.UserRepo
}

func RegisterRoutes(api huma.API, users repo.UserRepo, prefix string) {
	h := &Handler{users: users}
	group := huma.NewGroup(api, prefix)

	huma.Get(group, "/me", h.getMe)
	huma.Patch(group, "/me", h.updateMe)
	huma.Get(group, "/{userID}", h.getByID)
}

type getMeOutput struct {
	Body *model.User
}

func (h *Handler) getMe(ctx context.Context, _ *struct{}) (*getMeOutput, error) {
	return &getMeOutput{Body: middleware.UserFromCtx(ctx)}, nil
}

type getByIDInput struct {
	UserID string `path:"userID"`
}

type userOutput struct {
	Body *model.User
}

func (h *Handler) getByID(ctx context.Context, input *getByIDInput) (*userOutput, error) {
	u, err := h.users.GetByID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, huma.Error404NotFound("not found")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &userOutput{Body: u}, nil
}

type updateMeInputBody struct {
	Name  *string `json:"name,omitempty" minLength:"1"`
	Email *string `json:"email,omitempty" minLength:"1"`
}

type updateMeInput struct {
	Body updateMeInputBody
}

func (h *Handler) updateMe(ctx context.Context, input *updateMeInput) (*userOutput, error) {
	u := middleware.UserFromCtx(ctx)
	if input.Body.Name != nil {
		if strings.TrimSpace(*input.Body.Name) == "" {
			return nil, huma.Error422UnprocessableEntity("name cannot be blank")
		}
		u.Name = *input.Body.Name
	}
	if input.Body.Email != nil {
		if strings.TrimSpace(*input.Body.Email) == "" {
			return nil, huma.Error422UnprocessableEntity("email cannot be blank")
		}
		u.Email = *input.Body.Email
	}
	if err := h.users.Update(ctx, u); err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return nil, huma.Error409Conflict("email already in use")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &userOutput{Body: u}, nil
}
