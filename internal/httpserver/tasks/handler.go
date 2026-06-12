package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
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

type Handler struct {
	projects repo.ProjectRepo
	tasks    repo.TaskRepo
	tags     repo.TagRepo
	api      huma.API
}

func Register(api huma.API, projects repo.ProjectRepo, tasks repo.TaskRepo, tags repo.TagRepo) {
	h := &Handler{projects: projects, tasks: tasks, tags: tags, api: api}

	taskMW := taskCtxMW(h)

	huma.Register(api, huma.Operation{
		OperationID: "get-task", Method: http.MethodGet, Path: "/tasks/{taskID}",
		Middlewares: huma.Middlewares{taskMW},
	}, h.get)

	huma.Register(api, huma.Operation{
		OperationID: "update-task", Method: http.MethodPatch, Path: "/tasks/{taskID}",
		Middlewares: huma.Middlewares{taskMW},
	}, h.update)

	huma.Register(api, huma.Operation{
		OperationID: "delete-task", Method: http.MethodDelete, Path: "/tasks/{taskID}",
		Middlewares: huma.Middlewares{taskMW},
	}, h.delete)

	huma.Register(api, huma.Operation{
		OperationID: "complete-task", Method: http.MethodPost, Path: "/tasks/{taskID}/complete",
		Middlewares: huma.Middlewares{taskMW},
	}, h.completeTask)

	huma.Register(api, huma.Operation{
		OperationID: "list-subtasks", Method: http.MethodGet, Path: "/tasks/{taskID}/tasks",
		Middlewares: huma.Middlewares{taskMW},
	}, h.listSubtasks)

	huma.Register(api, huma.Operation{
		OperationID: "create-subtask", Method: http.MethodPost, Path: "/tasks/{taskID}/tasks",
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{taskMW},
	}, h.createSubtask)

	huma.Register(api, huma.Operation{
		OperationID: "list-task-tags", Method: http.MethodGet, Path: "/tasks/{taskID}/tags",
		Middlewares: huma.Middlewares{taskMW},
	}, h.listTags)

	huma.Register(api, huma.Operation{
		OperationID: "add-task-tag", Method: http.MethodPost, Path: "/tasks/{taskID}/tags",
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{taskMW},
	}, h.addTag)

	huma.Register(api, huma.Operation{
		OperationID: "delete-task-tag", Method: http.MethodDelete, Path: "/tasks/{taskID}/tags/{tag}",
		Middlewares: huma.Middlewares{taskMW},
	}, h.deleteTag)
}

func taskCtxMW(h *Handler) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, _ := humachi.Unwrap(ctx)
		id := chi.URLParam(r, "taskID")
		t, err := h.tasks.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = huma.WriteErr(h.api, ctx, http.StatusNotFound, "not found")
			} else {
				_ = huma.WriteErr(h.api, ctx, http.StatusInternalServerError, "internal error")
			}
			return
		}

		user := middleware.UserFromCtx(r.Context())
		role, err := h.projects.GetMemberRole(r.Context(), t.ProjectID, user.ID)
		if err != nil {
			_ = huma.WriteErr(h.api, ctx, http.StatusForbidden, "forbidden")
			return
		}

		newCtx := context.WithValue(r.Context(), taskCtxKey{}, t)
		newCtx = context.WithValue(newCtx, taskRoleCtxKey{}, role)
		next(huma.WithContext(ctx, newCtx))
	}
}

type getTaskOutput struct {
	Body model.Task
}

func (h *Handler) get(ctx context.Context, _ *struct{}) (*getTaskOutput, error) {
	return &getTaskOutput{Body: *taskFromCtx(ctx)}, nil
}

type updateTaskInput struct {
	TaskID string `path:"taskID"`
	Body   struct {
		Name        *string          `json:"name,omitempty"`
		Description *string          `json:"description,omitempty"`
		Status      *string          `json:"status,omitempty"`
		DueDate     *string          `json:"due_date,omitempty"`
		AssigneeID  *string          `json:"assignee_id,omitempty"`
		Position    *int             `json:"position,omitempty"`
		ParentID    nullable[string] `json:"parent_id,omitempty"`
		ProjectID   *string          `json:"project_id,omitempty"`
	}
}

type updateTaskOutput struct {
	Body model.Task
}

func (h *Handler) update(ctx context.Context, input *updateTaskInput) (*updateTaskOutput, error) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}

	t := taskFromCtx(ctx)

	if input.Body.ProjectID != nil && *input.Body.ProjectID != t.ProjectID {
		user := middleware.UserFromCtx(ctx)
		targetRole, err := h.projects.GetMemberRole(ctx, *input.Body.ProjectID, user.ID)
		if err != nil || !requireRole(model.RoleModify, targetRole) {
			return nil, huma.Error403Forbidden("forbidden")
		}
		t.ProjectID = *input.Body.ProjectID
	}

	if input.Body.Name != nil {
		t.Name = *input.Body.Name
	}
	if input.Body.Description != nil {
		t.Description = input.Body.Description
	}
	if input.Body.Status != nil {
		t.Status = *input.Body.Status
	}
	if input.Body.DueDate != nil {
		t.DueDate = input.Body.DueDate
	}
	if input.Body.AssigneeID != nil {
		t.AssigneeID = input.Body.AssigneeID
	}
	if input.Body.Position != nil {
		t.Position = *input.Body.Position
	}
	if input.Body.ParentID.Set {
		t.ParentID = input.Body.ParentID.Value
	}

	if err := h.tasks.Update(ctx, t); err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return nil, huma.Error409Conflict("invalid status")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &updateTaskOutput{Body: *t}, nil
}

func (h *Handler) delete(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	t := taskFromCtx(ctx)
	if err := h.tasks.Delete(ctx, t.ID); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

type completeTaskInput struct {
	TaskID string `path:"taskID"`
	Body   struct {
		DoneStatus string `json:"done_status,omitempty"`
	}
}

type completeTaskOutput struct {
	Body struct {
		Completed *model.Task `json:"completed"`
		Next      *model.Task `json:"next"`
	}
}

func (h *Handler) completeTask(ctx context.Context, input *completeTaskInput) (*completeTaskOutput, error) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.DoneStatus) == "" {
		return nil, huma.Error400BadRequest("done_status is required")
	}
	t := taskFromCtx(ctx)
	completed, next, err := h.tasks.CompleteTask(ctx, t.ID, input.Body.DoneStatus)
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return nil, huma.Error409Conflict("invalid done_status or missing due_date for recurring task")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &completeTaskOutput{Body: struct {
		Completed *model.Task `json:"completed"`
		Next      *model.Task `json:"next"`
	}{Completed: completed, Next: next}}, nil
}

type listSubtasksOutput struct {
	Body []*model.Task
}

func (h *Handler) listSubtasks(ctx context.Context, _ *struct{}) (*listSubtasksOutput, error) {
	t := taskFromCtx(ctx)
	parentID := t.ID
	list, err := h.tasks.ListChildren(ctx, t.ProjectID, &parentID, repo.TaskFilter{})
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &listSubtasksOutput{Body: list}, nil
}

type createSubtaskInput struct {
	TaskID string `path:"taskID"`
	Body   struct {
		Name        string  `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
		Status      *string `json:"status,omitempty"`
		DueDate     *string `json:"due_date,omitempty"`
		AssigneeID  *string `json:"assignee_id,omitempty"`
	}
}

type createSubtaskOutput struct {
	Body model.Task
}

func (h *Handler) createSubtask(ctx context.Context, input *createSubtaskInput) (*createSubtaskOutput, error) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Name) == "" {
		return nil, huma.Error400BadRequest("name is required")
	}
	parent := taskFromCtx(ctx)
	user := middleware.UserFromCtx(ctx)
	parentID := parent.ID
	status := "todo"
	if input.Body.Status != nil {
		status = *input.Body.Status
	}
	t := &model.Task{
		ID:          uuid.New().String(),
		ProjectID:   parent.ProjectID,
		ParentID:    &parentID,
		Name:        input.Body.Name,
		Description: input.Body.Description,
		Status:      status,
		DueDate:     input.Body.DueDate,
		OwnerID:     user.ID,
		AssigneeID:  input.Body.AssigneeID,
	}
	if err := h.tasks.Create(ctx, t); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &createSubtaskOutput{Body: *t}, nil
}

type listTagsOutput struct {
	Body []string
}

func (h *Handler) listTags(ctx context.Context, _ *struct{}) (*listTagsOutput, error) {
	t := taskFromCtx(ctx)
	list, err := h.tags.ListForTask(ctx, t.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if list == nil {
		list = []string{}
	}
	return &listTagsOutput{Body: list}, nil
}

type addTagInput struct {
	TaskID string `path:"taskID"`
	Body   struct {
		Tag string `json:"tag,omitempty"`
	}
}

type addTagOutput struct {
	Body struct {
		TaskID string `json:"task_id"`
		Tag    string `json:"tag"`
	}
}

func (h *Handler) addTag(ctx context.Context, input *addTagInput) (*addTagOutput, error) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Tag) == "" {
		return nil, huma.Error400BadRequest("tag is required")
	}
	t := taskFromCtx(ctx)
	if err := h.tags.Add(ctx, t.ID, input.Body.Tag); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &addTagOutput{Body: struct {
		TaskID string `json:"task_id"`
		Tag    string `json:"tag"`
	}{TaskID: t.ID, Tag: input.Body.Tag}}, nil
}

type deleteTagInput struct {
	TaskID string `path:"taskID"`
	Tag    string `path:"tag"`
}

func (h *Handler) deleteTag(ctx context.Context, input *deleteTagInput) (*struct{}, error) {
	if !requireRole(model.RoleModify, taskRoleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	t := taskFromCtx(ctx)
	if err := h.tags.Delete(ctx, t.ID, input.Tag); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}
