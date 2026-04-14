package projects

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

type projectCtxKey struct{}
type roleCtxKey struct{}

func projectFromCtx(ctx context.Context) *model.Project {
	p, _ := ctx.Value(projectCtxKey{}).(*model.Project)
	return p
}

func roleFromCtx(ctx context.Context) string {
	r, _ := ctx.Value(roleCtxKey{}).(string)
	return r
}

// Handler holds the projects repository dependency.
type Handler struct {
	projects repo.ProjectRepo
}

// NewRouter wires all /projects routes and returns the handler tree.
func NewRouter(projects repo.ProjectRepo) http.Handler {
	h := &Handler{projects: projects}
	r := chi.NewRouter()

	r.Get("/", h.list)
	r.Post("/", h.create)

	r.Route("/{projectID}", func(r chi.Router) {
		r.Use(h.projectCtx)
		r.Get("/", h.get)
		r.Patch("/", h.update)
		r.Delete("/", h.delete)

		r.Get("/members", h.listMembers)
		r.Post("/members", h.addMember)
		r.Patch("/members/{userID}", h.updateMember)
		r.Delete("/members/{userID}", h.removeMember)

		r.Get("/statuses", h.listStatuses)
		r.Post("/statuses", h.addStatus)
		r.Delete("/statuses/{status}", h.deleteStatus)

		r.Get("/tasks", h.listTasksStub)
		r.Post("/tasks", h.createTaskStub)
	})

	return r
}

// projectCtx loads the project and the caller's role into the request context.
func (h *Handler) projectCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "projectID")
		p, err := h.projects.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				render.NotFound(w)
			} else {
				render.Error(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		user := middleware.UserFromCtx(r.Context())
		role, err := h.projects.GetMemberRole(r.Context(), id, user.ID)
		if err != nil {
			render.Forbidden(w)
			return
		}

		ctx := context.WithValue(r.Context(), projectCtxKey{}, p)
		ctx = context.WithValue(ctx, roleCtxKey{}, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Projects CRUD ──────────────────────────────────────────────────────────

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromCtx(r.Context())
	list, err := h.projects.List(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, list)
}

type createProjectReq struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date"`
	AssigneeID  *string `json:"assignee_id"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body createProjectReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		render.BadRequest(w, "name is required")
		return
	}
	user := middleware.UserFromCtx(r.Context())
	p := &model.Project{
		ID:          uuid.New().String(),
		Name:        body.Name,
		Description: body.Description,
		DueDate:     body.DueDate,
		OwnerID:     user.ID,
		AssigneeID:  body.AssigneeID,
	}
	if err := h.projects.Create(r.Context(), p); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusCreated, p)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, projectFromCtx(r.Context()))
}

type updateProjectReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date"`
	AssigneeID  *string `json:"assignee_id"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleModify, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body updateProjectReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	p := projectFromCtx(r.Context())
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = body.Description
	}
	if body.DueDate != nil {
		p.DueDate = body.DueDate
	}
	if body.AssigneeID != nil {
		p.AssigneeID = body.AssigneeID
	}
	if err := h.projects.Update(r.Context(), p); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, p)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	p := projectFromCtx(r.Context())
	if err := h.projects.Delete(r.Context(), p.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.NoContent(w)
}

// ── Members ────────────────────────────────────────────────────────────────

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	p := projectFromCtx(r.Context())
	members, err := h.projects.ListMembers(r.Context(), p.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, members)
}

type addMemberReq struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body addMemberReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.UserID) == "" {
		render.BadRequest(w, "user_id is required")
		return
	}
	if !validRole(body.Role) {
		render.BadRequest(w, "role must be read, modify, or admin")
		return
	}
	caller := middleware.UserFromCtx(r.Context())
	if body.UserID == caller.ID {
		render.BadRequest(w, "cannot add yourself as a member")
		return
	}
	p := projectFromCtx(r.Context())
	m := &model.ProjectMember{ProjectID: p.ID, UserID: body.UserID, Role: body.Role}
	if err := h.projects.AddMember(r.Context(), m); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusCreated, m)
}

type updateMemberReq struct {
	Role string `json:"role"`
}

func (h *Handler) updateMember(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body updateMemberReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	if !validRole(body.Role) {
		render.BadRequest(w, "role must be read, modify, or admin")
		return
	}
	p := projectFromCtx(r.Context())
	userID := chi.URLParam(r, "userID")
	if userID == p.OwnerID {
		render.BadRequest(w, "cannot change role of project owner")
		return
	}
	if err := h.projects.UpdateMemberRole(r.Context(), p.ID, userID, body.Role); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, map[string]string{"project_id": p.ID, "user_id": userID, "role": body.Role})
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	p := projectFromCtx(r.Context())
	userID := chi.URLParam(r, "userID")
	if userID == p.OwnerID {
		render.BadRequest(w, "cannot remove project owner")
		return
	}
	if err := h.projects.RemoveMember(r.Context(), p.ID, userID); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.NoContent(w)
}

// ── Statuses ───────────────────────────────────────────────────────────────

func (h *Handler) listStatuses(w http.ResponseWriter, r *http.Request) {
	p := projectFromCtx(r.Context())
	statuses, err := h.projects.ListStatuses(r.Context(), p.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, statuses)
}

type addStatusReq struct {
	Status string `json:"status"`
}

func (h *Handler) addStatus(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body addStatusReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.Status) == "" {
		render.BadRequest(w, "status is required")
		return
	}
	p := projectFromCtx(r.Context())
	if err := h.projects.AddStatus(r.Context(), p.ID, body.Status); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusCreated, map[string]string{"project_id": p.ID, "status": body.Status})
}

func (h *Handler) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	p := projectFromCtx(r.Context())
	status := chi.URLParam(r, "status")
	err := h.projects.DeleteStatus(r.Context(), p.ID, status)
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			render.Error(w, http.StatusConflict, "status is in use by tasks")
			return
		}
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.NoContent(w)
}

// ── Task stubs (wired for real in PR 7) ───────────────────────────────────

func (h *Handler) listTasksStub(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, []*model.Task{})
}

func (h *Handler) createTaskStub(w http.ResponseWriter, _ *http.Request) {
	render.Error(w, http.StatusNotImplemented, "not implemented")
}
