package tags

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

type Handler struct {
	tags repo.TagRepo
}

func Register(api huma.API, tags repo.TagRepo) {
	h := &Handler{tags: tags}
	huma.Get(api, "/tags", h.listTags)
}

type listTagsOutput struct {
	Body []string
}

func (h *Handler) listTags(ctx context.Context, _ *struct{}) (*listTagsOutput, error) {
	user := middleware.UserFromCtx(ctx)
	list, err := h.tags.ListDistinctForUser(ctx, user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if list == nil {
		list = []string{}
	}
	return &listTagsOutput{Body: list}, nil
}
