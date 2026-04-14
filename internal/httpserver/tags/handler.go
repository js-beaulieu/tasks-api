package tags

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	"github.com/js-beaulieu/tasks/internal/repo"
)

// Handler holds the tags repository dependency.
type Handler struct{ tags repo.TagRepo }

// NewRouter wires the /tags routes and returns the handler tree.
func NewRouter(tags repo.TagRepo) http.Handler {
	h := &Handler{tags: tags}
	r := chi.NewRouter()
	r.Get("/", h.listTags)
	return r
}

// listTags returns all distinct tags belonging to projects the caller is a member of.
func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromCtx(r.Context())
	list, err := h.tags.ListDistinctForUser(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []string{}
	}
	render.JSON(w, http.StatusOK, list)
}
