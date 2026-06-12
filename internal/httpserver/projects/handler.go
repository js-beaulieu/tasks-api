package projects

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
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

type Handler struct {
	projects repo.ProjectRepo
	tasks    repo.TaskRepo
	api      huma.API
}

func Register(api huma.API, projects repo.ProjectRepo, tasks repo.TaskRepo) {
	h := &Handler{projects: projects, tasks: tasks, api: api}
	register(api, h, "/projects")
}

func NewRouter(projects repo.ProjectRepo, tasks repo.TaskRepo) http.Handler {
	r := chi.NewRouter()
	api := humachi.New(r, render.HumaConfig())
	register(api, &Handler{projects: projects, tasks: tasks, api: api}, "")
	return r
}

func register(api huma.API, h *Handler, prefix string) {

	huma.Register(api, huma.Operation{
		OperationID: "list-projects",
		Method:      http.MethodGet,
		Path:        route(prefix, "/"),
	}, h.list)

	huma.Register(api, huma.Operation{
		OperationID:   "create-project",
		Method:        http.MethodPost,
		Path:          route(prefix, "/"),
		DefaultStatus: http.StatusCreated,
	}, h.create)

	projectCtxMW := projectCtxMW(h)

	huma.Register(api, huma.Operation{
		OperationID: "get-project",
		Method:      http.MethodGet,
		Path:        route(prefix, "/{projectID}"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.get)

	huma.Register(api, huma.Operation{
		OperationID: "update-project",
		Method:      http.MethodPatch,
		Path:        route(prefix, "/{projectID}"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.update)

	huma.Register(api, huma.Operation{
		OperationID: "delete-project",
		Method:      http.MethodDelete,
		Path:        route(prefix, "/{projectID}"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.delete)

	huma.Register(api, huma.Operation{
		OperationID: "list-members",
		Method:      http.MethodGet,
		Path:        route(prefix, "/{projectID}/members"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.listMembers)

	huma.Register(api, huma.Operation{
		OperationID:   "add-member",
		Method:        http.MethodPost,
		Path:          route(prefix, "/{projectID}/members"),
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{projectCtxMW},
	}, h.addMember)

	huma.Register(api, huma.Operation{
		OperationID: "update-member",
		Method:      http.MethodPatch,
		Path:        route(prefix, "/{projectID}/members/{userID}"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.updateMember)

	huma.Register(api, huma.Operation{
		OperationID: "remove-member",
		Method:      http.MethodDelete,
		Path:        route(prefix, "/{projectID}/members/{userID}"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.removeMember)

	huma.Register(api, huma.Operation{
		OperationID: "list-statuses",
		Method:      http.MethodGet,
		Path:        route(prefix, "/{projectID}/statuses"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.listStatuses)

	huma.Register(api, huma.Operation{
		OperationID:   "add-status",
		Method:        http.MethodPost,
		Path:          route(prefix, "/{projectID}/statuses"),
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{projectCtxMW},
	}, h.addStatus)

	huma.Register(api, huma.Operation{
		OperationID: "delete-status",
		Method:      http.MethodDelete,
		Path:        route(prefix, "/{projectID}/statuses/{status}"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.deleteStatus)

	huma.Register(api, huma.Operation{
		OperationID: "list-project-tasks",
		Method:      http.MethodGet,
		Path:        route(prefix, "/{projectID}/tasks"),
		Middlewares: huma.Middlewares{projectCtxMW},
	}, h.listTasks)

	huma.Register(api, huma.Operation{
		OperationID:   "create-project-task",
		Method:        http.MethodPost,
		Path:          route(prefix, "/{projectID}/tasks"),
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{projectCtxMW},
	}, h.createTask)
}

func route(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "/" {
		return prefix
	}
	return prefix + path
}

func projectCtxMW(h *Handler) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, _ := humachi.Unwrap(ctx)
		id := chi.URLParam(r, "projectID")
		p, err := h.projects.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = huma.WriteErr(h.api, ctx, http.StatusNotFound, "not found")
			} else {
				_ = huma.WriteErr(h.api, ctx, http.StatusInternalServerError, "internal error")
			}
			return
		}

		user := middleware.UserFromCtx(r.Context())
		role, err := h.projects.GetMemberRole(r.Context(), id, user.ID)
		if err != nil {
			_ = huma.WriteErr(h.api, ctx, http.StatusForbidden, "forbidden")
			return
		}

		newCtx := context.WithValue(r.Context(), projectCtxKey{}, p)
		newCtx = context.WithValue(newCtx, roleCtxKey{}, role)
		next(huma.WithContext(ctx, newCtx))
	}
}

type listProjectsOutput struct {
	Body []*model.Project
}

func (h *Handler) list(ctx context.Context, _ *struct{}) (*listProjectsOutput, error) {
	user := middleware.UserFromCtx(ctx)
	list, err := h.projects.List(ctx, user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &listProjectsOutput{Body: list}, nil
}

type createProjectInput struct {
	Body struct {
		Name        string   `json:"name,omitempty"`
		Description *string  `json:"description,omitempty"`
		DueDate     *string  `json:"due_date,omitempty"`
		AssigneeID  *string  `json:"assignee_id,omitempty"`
		Statuses    []string `json:"statuses,omitempty"`
	}
}

type createProjectOutput struct {
	Body model.Project
}

func (h *Handler) create(ctx context.Context, input *createProjectInput) (*createProjectOutput, error) {
	if strings.TrimSpace(input.Body.Name) == "" {
		return nil, huma.Error400BadRequest("name is required")
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
	return &createProjectOutput{Body: *p}, nil
}

type getProjectOutput struct {
	Body model.Project
}

func (h *Handler) get(ctx context.Context, _ *struct{}) (*getProjectOutput, error) {
	return &getProjectOutput{Body: *projectFromCtx(ctx)}, nil
}

type updateProjectInput struct {
	ProjectID string `path:"projectID"`
	Body      struct {
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
		DueDate     *string `json:"due_date,omitempty"`
		AssigneeID  *string `json:"assignee_id,omitempty"`
	}
}

type updateProjectOutput struct {
	Body model.Project
}

func (h *Handler) update(ctx context.Context, input *updateProjectInput) (*updateProjectOutput, error) {
	if !RequireRole(model.RoleModify, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	p := projectFromCtx(ctx)
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
	return &updateProjectOutput{Body: *p}, nil
}

func (h *Handler) delete(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	p := projectFromCtx(ctx)
	if err := h.projects.Delete(ctx, p.ID); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

type listMembersOutput struct {
	Body []*model.ProjectMember
}

func (h *Handler) listMembers(ctx context.Context, _ *struct{}) (*listMembersOutput, error) {
	p := projectFromCtx(ctx)
	members, err := h.projects.ListMembers(ctx, p.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &listMembersOutput{Body: members}, nil
}

type addMemberInput struct {
	ProjectID string `path:"projectID"`
	Body      struct {
		UserID string `json:"user_id,omitempty"`
		Role   string `json:"role,omitempty"`
	}
}

type addMemberOutput struct {
	Body model.ProjectMember
}

func (h *Handler) addMember(ctx context.Context, input *addMemberInput) (*addMemberOutput, error) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.UserID) == "" {
		return nil, huma.Error400BadRequest("user_id is required")
	}
	if !validRole(input.Body.Role) {
		return nil, huma.Error400BadRequest("role must be read, modify, or admin")
	}
	caller := middleware.UserFromCtx(ctx)
	if input.Body.UserID == caller.ID {
		return nil, huma.Error400BadRequest("cannot add yourself as a member")
	}
	p := projectFromCtx(ctx)
	m := &model.ProjectMember{ProjectID: p.ID, UserID: input.Body.UserID, Role: input.Body.Role}
	if err := h.projects.AddMember(ctx, m); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &addMemberOutput{Body: *m}, nil
}

type updateMemberInput struct {
	ProjectID string `path:"projectID"`
	UserID    string `path:"userID"`
	Body      struct {
		Role string `json:"role,omitempty"`
	}
}

type updateMemberOutput struct {
	Body struct {
		ProjectID string `json:"project_id"`
		UserID    string `json:"user_id"`
		Role      string `json:"role"`
	}
}

func (h *Handler) updateMember(ctx context.Context, input *updateMemberInput) (*updateMemberOutput, error) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if !validRole(input.Body.Role) {
		return nil, huma.Error400BadRequest("role must be read, modify, or admin")
	}
	p := projectFromCtx(ctx)
	if input.UserID == p.OwnerID {
		return nil, huma.Error400BadRequest("cannot change role of project owner")
	}
	if err := h.projects.UpdateMemberRole(ctx, p.ID, input.UserID, input.Body.Role); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &updateMemberOutput{Body: struct {
		ProjectID string `json:"project_id"`
		UserID    string `json:"user_id"`
		Role      string `json:"role"`
	}{ProjectID: p.ID, UserID: input.UserID, Role: input.Body.Role}}, nil
}

type removeMemberInput struct {
	ProjectID string `path:"projectID"`
	UserID    string `path:"userID"`
}

func (h *Handler) removeMember(ctx context.Context, input *removeMemberInput) (*struct{}, error) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	p := projectFromCtx(ctx)
	if input.UserID == p.OwnerID {
		return nil, huma.Error400BadRequest("cannot remove project owner")
	}
	if err := h.projects.RemoveMember(ctx, p.ID, input.UserID); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

type listStatusesOutput struct {
	Body []*model.ProjectStatus
}

func (h *Handler) listStatuses(ctx context.Context, _ *struct{}) (*listStatusesOutput, error) {
	p := projectFromCtx(ctx)
	statuses, err := h.projects.ListStatuses(ctx, p.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &listStatusesOutput{Body: statuses}, nil
}

type addStatusInput struct {
	ProjectID string `path:"projectID"`
	Body      struct {
		Status string `json:"status,omitempty"`
	}
}

type addStatusOutput struct {
	Body struct {
		ProjectID string `json:"project_id"`
		Status    string `json:"status"`
	}
}

func (h *Handler) addStatus(ctx context.Context, input *addStatusInput) (*addStatusOutput, error) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Status) == "" {
		return nil, huma.Error400BadRequest("status is required")
	}
	p := projectFromCtx(ctx)
	if err := h.projects.AddStatus(ctx, p.ID, input.Body.Status); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &addStatusOutput{Body: struct {
		ProjectID string `json:"project_id"`
		Status    string `json:"status"`
	}{ProjectID: p.ID, Status: input.Body.Status}}, nil
}

type deleteStatusInput struct {
	ProjectID string `path:"projectID"`
	Status    string `path:"status"`
}

func (h *Handler) deleteStatus(ctx context.Context, input *deleteStatusInput) (*struct{}, error) {
	if !RequireRole(model.RoleAdmin, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	p := projectFromCtx(ctx)
	err := h.projects.DeleteStatus(ctx, p.ID, input.Status)
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return nil, huma.Error409Conflict("status is in use by tasks")
		}
		return nil, huma.Error500InternalServerError("internal error")
	}
	return nil, nil
}

type listTasksOutput struct {
	Body []*model.Task
}

func (h *Handler) listTasks(ctx context.Context, input *struct {
	ProjectID  string `path:"projectID"`
	Status     string `query:"status"`
	AssigneeID string `query:"assignee_id"`
	Tag        string `query:"tag"`
}) (*listTasksOutput, error) {
	p := projectFromCtx(ctx)
	var f repo.TaskFilter
	if input != nil {
		if input.Status != "" {
			f.Status = &input.Status
		}
		if input.AssigneeID != "" {
			f.AssigneeID = &input.AssigneeID
		}
		if input.Tag != "" {
			f.Tag = &input.Tag
		}
	}
	list, err := h.tasks.ListChildren(ctx, p.ID, nil, f)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &listTasksOutput{Body: list}, nil
}

type createTaskInput struct {
	ProjectID string `path:"projectID"`
	Body      struct {
		Name        string  `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
		Status      *string `json:"status,omitempty"`
		DueDate     *string `json:"due_date,omitempty"`
		AssigneeID  *string `json:"assignee_id,omitempty"`
	}
}

type createTaskOutput struct {
	Body model.Task
}

func (h *Handler) createTask(ctx context.Context, input *createTaskInput) (*createTaskOutput, error) {
	if !RequireRole(model.RoleModify, roleFromCtx(ctx)) {
		return nil, huma.Error403Forbidden("forbidden")
	}
	if strings.TrimSpace(input.Body.Name) == "" {
		return nil, huma.Error400BadRequest("name is required")
	}
	p := projectFromCtx(ctx)
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
	}
	if err := h.tasks.Create(ctx, t); err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}
	return &createTaskOutput{Body: *t}, nil
}
