package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/humautil"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
)

type Handler struct {
	projects repo.ProjectRepo
	tasks    repo.TaskRepo
	tags     repo.TagRepo
}

func RegisterRoutes(api huma.API, projects repo.ProjectRepo, tasks repo.TaskRepo, tags repo.TagRepo, prefix string) {
	h := &Handler{projects: projects, tasks: tasks, tags: tags}
	group := huma.NewGroup(api, prefix)

	huma.Get(group, "/{taskID}", h.get)
	huma.Patch(group, "/{taskID}", h.update)
	huma.Delete(group, "/{taskID}", h.delete)
	huma.Get(group, "/{taskID}/tasks", h.listSubtasks)
	huma.Post(group, "/{taskID}/tasks", h.createSubtask)
	huma.Get(group, "/{taskID}/tags", h.listTags)
	huma.Post(group, "/{taskID}/tags", h.addTag)
	huma.Delete(group, "/{taskID}/tags/{tag}", h.deleteTag)
}

func (h *Handler) loadTask(ctx context.Context, taskID string) (*model.Task, string, error) {
	t, err := h.tasks.Get(ctx, taskID)
	if err != nil {
		return nil, "", err
	}
	user := middleware.UserFromCtx(ctx)
	role, err := h.projects.GetMemberRole(ctx, t.ProjectID, user.ID)
	if err != nil {
		return nil, "", repo.ErrNoAccess
	}
	return t, role, nil
}

type taskInput struct {
	TaskID string `path:"taskID"`
}

type taskOutput struct {
	Body *model.Task
}

func (h *Handler) get(ctx context.Context, input *taskInput) (*taskOutput, error) {
	t, _, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	return &taskOutput{Body: t}, nil
}

type nullable[T any] struct {
	Value *T
	Set   bool
}

func (nullable[T]) TransformSchema(r huma.Registry, s *huma.Schema) *huma.Schema {
	s.Type = "string"
	s.Nullable = true
	s.Properties = nil
	s.Required = nil
	s.AdditionalProperties = nil
	s.Ref = ""
	return s
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

type updateTaskBody struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Status      *string          `json:"status,omitempty"`
	DueDate     *string          `json:"due_date,omitempty"`
	AssigneeID  *string          `json:"assignee_id,omitempty"`
	Position    *int             `json:"position,omitempty"`
	ParentID    nullable[string] `json:"parent_id,omitempty"`
	ProjectID   *string          `json:"project_id,omitempty"`
	Recurrence  nullable[string] `json:"recurrence,omitempty"`
}

type updateTaskInput struct {
	TaskID string `path:"taskID"`
	Body   updateTaskBody
}

type updateTaskOutput struct {
	NextOccurrenceID string `header:"X-Next-Occurrence-Id"`
	Body             *model.Task
}

func (h *Handler) update(ctx context.Context, input *updateTaskInput) (*updateTaskOutput, error) {
	t, role, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	if !humautil.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if input.Body.ProjectID != nil && *input.Body.ProjectID != t.ProjectID {
		user := middleware.UserFromCtx(ctx)
		targetRole, err := h.projects.GetMemberRole(ctx, *input.Body.ProjectID, user.ID)
		if err != nil || !humautil.RequireRole(model.RoleModify, targetRole) {
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
	oldStatus := t.Status
	if input.Body.Status != nil {
		t.Status = *input.Body.Status
	}
	if input.Body.Position != nil {
		t.Position = *input.Body.Position
	} else if input.Body.Status != nil && *input.Body.Status != oldStatus {
		siblingCount, err := h.tasks.ListChildren(ctx, t.ProjectID, t.ParentID, repo.TaskFilter{Status: input.Body.Status})
		if err != nil {
			return nil, huma.Error500InternalServerError("internal error")
		}
		t.Position = len(siblingCount)
	}
	if input.Body.DueDate != nil {
		t.DueDate = input.Body.DueDate
	}
	if input.Body.AssigneeID != nil {
		t.AssigneeID = input.Body.AssigneeID
	}
	if input.Body.ParentID.Set {
		t.ParentID = input.Body.ParentID.Value
	}
	if input.Body.Recurrence.Set {
		if err := humautil.ValidateRecurrence(input.Body.Recurrence.Value, t.DueDate); err != nil {
			return nil, err
		}
		t.Recurrence = input.Body.Recurrence.Value
	}
	// Pre-validation: recurring task being marked done must have a due_date.
	if t.Status == "done" && t.Recurrence != nil && *t.Recurrence != "" && t.DueDate == nil {
		return nil, huma.Error409Conflict("recurring task requires due_date to complete")
	}
	updated, nextID, err := h.tasks.Update(ctx, t)
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return nil, huma.Error409Conflict("invalid status or missing due_date for recurring task")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	out := &updateTaskOutput{Body: updated}
	if nextID != nil {
		out.NextOccurrenceID = *nextID
	}
	return out, nil
}

func (h *Handler) delete(ctx context.Context, input *taskInput) (*struct{}, error) {
	t, role, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	if !humautil.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if err := h.tasks.Delete(ctx, t.ID); err != nil {
		return nil, humautil.RepoError(err)
	}
	return nil, nil
}

type subtaskListOutput struct {
	Body []*model.Task
}

func (h *Handler) listSubtasks(ctx context.Context, input *taskInput) (*subtaskListOutput, error) {
	t, _, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	parentID := t.ID
	list, err := h.tasks.ListChildren(ctx, t.ProjectID, &parentID, repo.TaskFilter{})
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if list == nil {
		list = []*model.Task{}
	}
	return &subtaskListOutput{Body: list}, nil
}

type createSubtaskBody struct {
	Name        string  `json:"name" minLength:"1"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	Recurrence  *string `json:"recurrence,omitempty"`
}

type createSubtaskInput struct {
	TaskID string `path:"taskID"`
	Body   createSubtaskBody
}

type createdTaskOutput struct {
	Status int `status:"201"`
	Body   *model.Task
}

func (h *Handler) createSubtask(ctx context.Context, input *createSubtaskInput) (*createdTaskOutput, error) {
	parent, role, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	if !humautil.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Name) == "" {
		return nil, huma.Error422UnprocessableEntity("name is required")
	}
	if err := humautil.ValidateRecurrence(input.Body.Recurrence, input.Body.DueDate); err != nil {
		return nil, err
	}
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
		Recurrence:  input.Body.Recurrence,
	}
	if err := h.tasks.Create(ctx, t); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &createdTaskOutput{Status: http.StatusCreated, Body: t}, nil
}

type tagListOutput struct {
	Body []string
}

func (h *Handler) listTags(ctx context.Context, input *taskInput) (*tagListOutput, error) {
	t, _, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	list, err := h.tags.ListForTask(ctx, t.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if list == nil {
		list = []string{}
	}
	return &tagListOutput{Body: list}, nil
}

type addTagBody struct {
	Tag string `json:"tag" minLength:"1"`
}

type addTagInput struct {
	TaskID string `path:"taskID"`
	Body   addTagBody
}

type tagOutput struct {
	Status int `status:"201"`
	Body   map[string]string
}

func (h *Handler) addTag(ctx context.Context, input *addTagInput) (*tagOutput, error) {
	t, role, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	if !humautil.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Tag) == "" {
		return nil, huma.Error422UnprocessableEntity("tag is required")
	}
	if err := h.tags.Add(ctx, t.ID, input.Body.Tag); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &tagOutput{Status: http.StatusCreated, Body: map[string]string{"task_id": t.ID, "tag": input.Body.Tag}}, nil
}

type deleteTagInput struct {
	TaskID string `path:"taskID"`
	Tag    string `path:"tag"`
}

func (h *Handler) deleteTag(ctx context.Context, input *deleteTagInput) (*struct{}, error) {
	t, role, err := h.loadTask(ctx, input.TaskID)
	if err != nil {
		return nil, humautil.RepoError(err)
	}
	if !humautil.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if err := h.tags.Delete(ctx, t.ID, input.Tag); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}
