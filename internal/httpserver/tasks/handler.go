package tasks

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

type taskCtxKey struct{}
type taskRoleCtxKey struct{}

func taskFromCtx(ctx context.Context) *model.Task {
	t, _ := ctx.Value(taskCtxKey{}).(*model.Task)
	return t
}

func taskRoleFromCtx(ctx context.Context) string {
	r, _ := ctx.Value(taskRoleCtxKey{}).(string)
	return r
}

var roleRank = map[string]int{
	model.RoleRead:   1,
	model.RoleModify: 2,
	model.RoleAdmin:  3,
}

func requireRole(min, actual string) bool {
	return roleRank[actual] >= roleRank[min]
}

// Handler holds all repository dependencies for task operations.
type Handler struct {
	projects repo.ProjectRepo
	tasks    repo.TaskRepo
	tags     repo.TagRepo
}

// NewRouter wires all /tasks/{taskID} routes and returns the handler tree.
func NewRouter(projects repo.ProjectRepo, tasks repo.TaskRepo, tags repo.TagRepo) http.Handler {
	h := &Handler{projects: projects, tasks: tasks, tags: tags}
	r := chi.NewRouter()

	r.Route("/{taskID}", func(r chi.Router) {
		r.Use(h.taskCtx)
		r.Get("/", h.get)
		r.Patch("/", h.update)
		r.Delete("/", h.delete)
		r.Get("/tasks", h.listSubtasks)
		r.Post("/tasks", h.createSubtask)
		r.Get("/tags", h.listTags)
		r.Post("/tags", h.addTag)
		r.Delete("/tags/{tag}", h.deleteTag)
	})

	return r
}

// taskCtx loads the task and the caller's role into the request context.
func (h *Handler) taskCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "taskID")
		t, err := h.tasks.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				render.NotFound(w)
			} else {
				render.Error(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		user := middleware.UserFromCtx(r.Context())
		role, err := h.projects.GetMemberRole(r.Context(), t.ProjectID, user.ID)
		if err != nil {
			render.Forbidden(w)
			return
		}

		ctx := context.WithValue(r.Context(), taskCtxKey{}, t)
		ctx = context.WithValue(ctx, taskRoleCtxKey{}, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Task CRUD ──────────────────────────────────────────────────────────────

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, taskFromCtx(r.Context()))
}

// nullable distinguishes "field omitted" (Set=false) from "field present but null"
// (Set=true, Value=nil) in JSON payloads.
type nullable[T any] struct {
	Value *T
	Set   bool
}

func (n *nullable[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.Value = &v
	return nil
}

type updateTaskReq struct {
	Name        *string          `json:"name"`
	Description *string          `json:"description"`
	Status      *string          `json:"status"`
	DueDate     *string          `json:"due_date"`
	AssigneeID  *string          `json:"assignee_id"`
	Position    *int             `json:"position"`
	ParentID    nullable[string] `json:"parent_id"`
	ProjectID   *string          `json:"project_id"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body updateTaskReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}

	t := taskFromCtx(r.Context())

	// Cross-project move: caller must have modify on the target project too.
	if body.ProjectID != nil && *body.ProjectID != t.ProjectID {
		user := middleware.UserFromCtx(r.Context())
		targetRole, err := h.projects.GetMemberRole(r.Context(), *body.ProjectID, user.ID)
		if err != nil || !requireRole(model.RoleModify, targetRole) {
			render.Forbidden(w)
			return
		}
		t.ProjectID = *body.ProjectID
	}

	if body.Name != nil {
		t.Name = *body.Name
	}
	if body.Description != nil {
		t.Description = body.Description
	}
	if body.Status != nil {
		t.Status = *body.Status
	}
	if body.DueDate != nil {
		t.DueDate = body.DueDate
	}
	if body.AssigneeID != nil {
		t.AssigneeID = body.AssigneeID
	}
	if body.Position != nil {
		t.Position = *body.Position
	}
	if body.ParentID.Set {
		t.ParentID = body.ParentID.Value
	}

	if err := h.tasks.Update(r.Context(), t); err != nil {
		if errors.Is(err, repo.ErrConflict) {
			render.Error(w, http.StatusConflict, "invalid status")
			return
		}
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	t := taskFromCtx(r.Context())
	if err := h.tasks.Delete(r.Context(), t.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.NoContent(w)
}

// ── Subtasks ───────────────────────────────────────────────────────────────

func (h *Handler) listSubtasks(w http.ResponseWriter, r *http.Request) {
	t := taskFromCtx(r.Context())
	parentID := t.ID
	list, err := h.tasks.ListChildren(r.Context(), t.ProjectID, &parentID, repo.TaskFilter{})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusOK, list)
}

type createTaskReq struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	DueDate     *string `json:"due_date"`
	AssigneeID  *string `json:"assignee_id"`
}

func (h *Handler) createSubtask(w http.ResponseWriter, r *http.Request) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body createTaskReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		render.BadRequest(w, "name is required")
		return
	}
	parent := taskFromCtx(r.Context())
	user := middleware.UserFromCtx(r.Context())
	parentID := parent.ID
	status := "todo"
	if body.Status != nil {
		status = *body.Status
	}
	t := &model.Task{
		ID:          uuid.New().String(),
		ProjectID:   parent.ProjectID,
		ParentID:    &parentID,
		Name:        body.Name,
		Description: body.Description,
		Status:      status,
		DueDate:     body.DueDate,
		OwnerID:     user.ID,
		AssigneeID:  body.AssigneeID,
	}
	if err := h.tasks.Create(r.Context(), t); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusCreated, t)
}

// ── Tags ───────────────────────────────────────────────────────────────────

func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	t := taskFromCtx(r.Context())
	list, err := h.tags.ListForTask(r.Context(), t.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = []string{}
	}
	render.JSON(w, http.StatusOK, list)
}

type addTagReq struct {
	Tag string `json:"tag"`
}

func (h *Handler) addTag(w http.ResponseWriter, r *http.Request) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	var body addTagReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.BadRequest(w, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.Tag) == "" {
		render.BadRequest(w, "tag is required")
		return
	}
	t := taskFromCtx(r.Context())
	if err := h.tags.Add(r.Context(), t.ID, body.Tag); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.JSON(w, http.StatusCreated, map[string]string{"task_id": t.ID, "tag": body.Tag})
}

func (h *Handler) deleteTag(w http.ResponseWriter, r *http.Request) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(r.Context())) {
		render.Forbidden(w)
		return
	}
	t := taskFromCtx(r.Context())
	tag := chi.URLParam(r, "tag")
	if err := h.tags.Delete(r.Context(), t.ID, tag); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	render.NoContent(w)
}
