package tags

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

type Handler struct {
	tags repo.TagRepo
}

func Register(api huma.API, tags repo.TagRepo) {
	h := &Handler{tags: tags}
	register(api, h, "/tags")
}

func NewRouter(tags repo.TagRepo) http.Handler {
	r := chi.NewRouter()
	api := humachi.New(r, render.HumaConfig())
	register(api, &Handler{tags: tags}, "")
	return r
}

func register(api huma.API, h *Handler, prefix string) {
	path := prefix
	if path == "" {
		path = "/"
	}
	huma.Get(api, path, h.listTags)
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
