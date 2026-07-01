package projects

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/access"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/recurrence"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
	repoerr "github.com/js-beaulieu/hs-api/libs/hs-common/repo"
)

func isPermanentStatus(status string) bool {
	for _, s := range model.PermanentStatuses {
		if s == status {
			return true
		}
	}
	return false
}

type Handler struct {
	projects repo.ProjectRepo
	tasks    repo.TaskRepo
}

func RegisterRoutes(api huma.API, projects repo.ProjectRepo, tasks repo.TaskRepo, prefix string) {
	h := &Handler{projects: projects, tasks: tasks}
	group := huma.NewGroup(api, prefix)

	huma.Get(group, rootPath(prefix), h.list)
	huma.Post(group, rootPath(prefix), h.create)
	huma.Get(group, "/{projectID}", h.get)
	huma.Patch(group, "/{projectID}", h.update)
	huma.Delete(group, "/{projectID}", h.delete)
	huma.Get(group, "/{projectID}/members", h.listMembers)
	huma.Post(group, "/{projectID}/members", h.addMember)
	huma.Patch(group, "/{projectID}/members/{userID}", h.updateMember)
	huma.Delete(group, "/{projectID}/members/{userID}", h.removeMember)
	huma.Get(group, "/{projectID}/statuses", h.listStatuses)
	huma.Post(group, "/{projectID}/statuses", h.addStatus)
	huma.Delete(group, "/{projectID}/statuses/{status}", h.deleteStatus)
	huma.Get(group, "/{projectID}/tasks", h.listTasks)
	huma.Post(group, "/{projectID}/tasks", h.createTask)
}

func rootPath(prefix string) string {
	if prefix == "" {
		return "/"
	}
	return ""
}

func (h *Handler) loadProject(ctx context.Context, projectID string) (*model.Project, string, error) {
	p, err := h.projects.Get(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	user := middleware.UserFromCtx(ctx)
	role, err := h.projects.GetMemberRole(ctx, projectID, user.ID)
	if err != nil {
		return nil, "", repoerr.ErrNoAccess
	}
	return p, role, nil
}

type projectOutput struct {
	Body *model.Project
}

type projectListOutput struct {
	Body []*model.Project
}

func (h *Handler) list(ctx context.Context, _ *struct{}) (*projectListOutput, error) {
	user := middleware.UserFromCtx(ctx)
	list, err := h.projects.List(ctx, user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if list == nil {
		list = []*model.Project{}
	}
	return &projectListOutput{Body: list}, nil
}

type createProjectBody struct {
	Name        string   `json:"name" minLength:"1"`
	Description *string  `json:"description,omitempty"`
	DueDate     *string  `json:"due_date,omitempty"`
	AssigneeID  *string  `json:"assignee_id,omitempty"`
	Statuses    []string `json:"statuses,omitempty"`
}

type createProjectInput struct {
	Body createProjectBody
}

type createdProjectOutput struct {
	Status int `status:"201"`
	Body   *model.Project
}

func (h *Handler) create(ctx context.Context, input *createProjectInput) (*createdProjectOutput, error) {
	if strings.TrimSpace(input.Body.Name) == "" {
		return nil, huma.Error422UnprocessableEntity("name is required")
	}
	user := middleware.UserFromCtx(ctx)
	p := &model.Project{
		ID:          uuid.New().String(),
		Name:        input.Body.Name,
		Description: input.Body.Description,
		DueDate:     input.Body.DueDate,
		OwnerID:     user.ID,
		AssigneeID:  input.Body.AssigneeID,
	}
	if err := h.projects.Create(ctx, p, input.Body.Statuses...); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	p.EffectiveRole = model.RoleAdmin
	return &createdProjectOutput{Status: http.StatusCreated, Body: p}, nil
}

type projectInput struct {
	ProjectID string `path:"projectID"`
}

func (h *Handler) get(ctx context.Context, input *projectInput) (*projectOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	p.EffectiveRole = role
	return &projectOutput{Body: p}, nil
}

type updateProjectBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
}

type updateProjectInput struct {
	ProjectID string `path:"projectID"`
	Body      updateProjectBody
}

func (h *Handler) update(ctx context.Context, input *updateProjectInput) (*projectOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if input.Body.Name != nil {
		p.Name = *input.Body.Name
	}
	if input.Body.Description != nil {
		p.Description = input.Body.Description
	}
	if input.Body.DueDate != nil {
		p.DueDate = input.Body.DueDate
	}
	if input.Body.AssigneeID != nil {
		p.AssigneeID = input.Body.AssigneeID
	}
	if err := h.projects.Update(ctx, p); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	p.EffectiveRole = role
	return &projectOutput{Body: p}, nil
}

func (h *Handler) delete(ctx context.Context, input *projectInput) (*struct{}, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleAdmin, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if err := h.projects.Delete(ctx, p.ID); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

type memberListOutput struct {
	Body []*model.ProjectMember
}

func (h *Handler) listMembers(ctx context.Context, input *projectInput) (*memberListOutput, error) {
	p, _, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	members, err := h.projects.ListMembers(ctx, p.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if members == nil {
		members = []*model.ProjectMember{}
	}
	return &memberListOutput{Body: members}, nil
}

type addMemberBody struct {
	UserID string `json:"user_id" minLength:"1"`
	Role   string `json:"role"`
}

type addMemberInput struct {
	ProjectID string `path:"projectID"`
	Body      addMemberBody
}

type createdMemberOutput struct {
	Status int `status:"201"`
	Body   *model.ProjectMember
}

func (h *Handler) addMember(ctx context.Context, input *addMemberInput) (*createdMemberOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleAdmin, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.UserID) == "" {
		return nil, huma.Error422UnprocessableEntity("user_id is required")
	}
	if !access.ValidRole(input.Body.Role) {
		return nil, huma.Error422UnprocessableEntity("role must be read, modify, or admin")
	}
	caller := middleware.UserFromCtx(ctx)
	if input.Body.UserID == caller.ID {
		return nil, huma.Error422UnprocessableEntity("cannot add yourself as a member")
	}
	m := &model.ProjectMember{ProjectID: p.ID, UserID: input.Body.UserID, Role: input.Body.Role}
	if err := h.projects.AddMember(ctx, m); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &createdMemberOutput{Status: http.StatusCreated, Body: m}, nil
}

type updateMemberBody struct {
	Role string `json:"role"`
}

type updateMemberInput struct {
	ProjectID string `path:"projectID"`
	UserID    string `path:"userID"`
	Body      updateMemberBody
}

type memberOutput struct {
	Body map[string]string
}

func (h *Handler) updateMember(ctx context.Context, input *updateMemberInput) (*memberOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleAdmin, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if !access.ValidRole(input.Body.Role) {
		return nil, huma.Error422UnprocessableEntity("role must be read, modify, or admin")
	}
	if input.UserID == p.OwnerID {
		return nil, huma.Error422UnprocessableEntity("cannot change role of project owner")
	}
	if err := h.projects.UpdateMemberRole(ctx, p.ID, input.UserID, input.Body.Role); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &memberOutput{Body: map[string]string{"project_id": p.ID, "user_id": input.UserID, "role": input.Body.Role}}, nil
}

type removeMemberInput struct {
	ProjectID string `path:"projectID"`
	UserID    string `path:"userID"`
}

type removeMemberOutput struct {
	Body struct {
		Reassigned int `json:"reassigned"`
	}
}

func (h *Handler) removeMember(ctx context.Context, input *removeMemberInput) (*removeMemberOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleAdmin, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if input.UserID == p.OwnerID {
		return nil, huma.Error422UnprocessableEntity("cannot remove project owner")
	}
	reassigned, err := h.projects.RemoveMember(ctx, p.ID, input.UserID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	out := &removeMemberOutput{}
	out.Body.Reassigned = reassigned
	return out, nil
}

type statusListOutput struct {
	Body []*model.ProjectStatus
}

func (h *Handler) listStatuses(ctx context.Context, input *projectInput) (*statusListOutput, error) {
	p, _, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	statuses, err := h.projects.ListStatuses(ctx, p.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if statuses == nil {
		statuses = []*model.ProjectStatus{}
	}
	return &statusListOutput{Body: statuses}, nil
}

type addStatusBody struct {
	Status string `json:"status" minLength:"1"`
}

type addStatusInput struct {
	ProjectID string `path:"projectID"`
	Body      addStatusBody
}

type statusOutput struct {
	Status int `status:"201"`
	Body   map[string]string
}

func (h *Handler) addStatus(ctx context.Context, input *addStatusInput) (*statusOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleAdmin, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Status) == "" {
		return nil, huma.Error422UnprocessableEntity("status is required")
	}
	if err := h.projects.AddStatus(ctx, p.ID, input.Body.Status); err != nil {
		if errors.Is(err, repoerr.ErrConflict) {
			return nil, huma.Error409Conflict("status already exists")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &statusOutput{Status: http.StatusCreated, Body: map[string]string{"project_id": p.ID, "status": input.Body.Status}}, nil
}

type deleteStatusInput struct {
	ProjectID string `path:"projectID"`
	Status    string `path:"status"`
}

func (h *Handler) deleteStatus(ctx context.Context, input *deleteStatusInput) (*struct{}, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleAdmin, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if isPermanentStatus(input.Status) {
		return nil, huma.Error409Conflict("cannot delete permanent status")
	}
	err = h.projects.DeleteStatus(ctx, p.ID, input.Status)
	if err != nil {
		if errors.Is(err, repoerr.ErrConflict) {
			return nil, huma.Error409Conflict("status is in use by tasks")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

type listTasksInput struct {
	ProjectID  string `path:"projectID"`
	Status     string `query:"status"`
	AssigneeID string `query:"assignee_id"`
	Tag        string `query:"tag"`
}

type taskListOutput struct {
	Body []*model.Task
}

func (h *Handler) listTasks(ctx context.Context, input *listTasksInput) (*taskListOutput, error) {
	p, _, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	list, err := h.tasks.ListChildren(ctx, p.ID, nil, repo.TaskFilter{
		Status:     stringPtr(input.Status),
		AssigneeID: stringPtr(input.AssigneeID),
		Tag:        stringPtr(input.Tag),
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	if list == nil {
		list = []*model.Task{}
	}
	return &taskListOutput{Body: list}, nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type createTaskBody struct {
	Name        string  `json:"name" minLength:"1"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	Recurrence  *string `json:"recurrence,omitempty"`
}

type createTaskInput struct {
	ProjectID string `path:"projectID"`
	Body      createTaskBody
}

type createdTaskOutput struct {
	Status int `status:"201"`
	Body   *model.Task
}

func (h *Handler) createTask(ctx context.Context, input *createTaskInput) (*createdTaskOutput, error) {
	p, role, err := h.loadProject(ctx, input.ProjectID)
	if err != nil {
		return nil, repoError(err)
	}
	if !access.RequireRole(model.RoleModify, role) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Name) == "" {
		return nil, huma.Error422UnprocessableEntity("name is required")
	}
	if err := recurrence.Validate(input.Body.Recurrence, input.Body.DueDate); err != nil {
		return nil, err
	}
	user := middleware.UserFromCtx(ctx)
	status := "todo"
	if input.Body.Status != nil {
		status = *input.Body.Status
	}
	t := &model.Task{
		ID:          uuid.New().String(),
		ProjectID:   p.ID,
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
func repoError(err error) error {
	if errors.Is(err, repoerr.ErrNotFound) {
		return huma.Error404NotFound("not found")
	}
	if errors.Is(err, repoerr.ErrNoAccess) {
		return huma.Error403Forbidden("forbidden")
	}
	return huma.Error500InternalServerError("internal error")
}
