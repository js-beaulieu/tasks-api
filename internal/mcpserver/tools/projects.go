package tools

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

// ── list_projects ─────────────────────────────────────────────────────────────

var ListProjectsTool = &mcp.Tool{
	Name:        "list_projects",
	Description: "List all projects visible to the given user.",
}

type listProjectsInput struct {
	UserID string `json:"user_id"`
}

type listProjectsResult struct {
	Projects []*model.Project `json:"projects"`
}

func ListProjectsHandler(projects repo.ProjectRepo) mcp.ToolHandlerFor[listProjectsInput, *listProjectsResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listProjectsInput) (*mcp.CallToolResult, *listProjectsResult, error) {
		if in.UserID == "" {
			return nil, nil, errors.New("user_id is required")
		}
		list, err := projects.List(ctx, in.UserID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &listProjectsResult{Projects: list}, nil
	}
}

// ── get_project ───────────────────────────────────────────────────────────────

var GetProjectTool = &mcp.Tool{
	Name:        "get_project",
	Description: "Get a project by ID.",
}

type getProjectInput struct {
	ProjectID string `json:"project_id"`
}

func GetProjectHandler(projects repo.ProjectRepo) mcp.ToolHandlerFor[getProjectInput, *model.Project] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getProjectInput) (*mcp.CallToolResult, *model.Project, error) {
		if in.ProjectID == "" {
			return nil, nil, errors.New("project_id is required")
		}
		p, err := projects.Get(ctx, in.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	}
}

// ── create_project ────────────────────────────────────────────────────────────

var CreateProjectTool = &mcp.Tool{
	Name:        "create_project",
	Description: "Create a new project. Default statuses are always added; pass extra statuses to seed custom workflow steps alongside them.",
}

type createProjectInput struct {
	UserID      string   `json:"user_id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	DueDate     *string  `json:"due_date,omitempty"`
	AssigneeID  *string  `json:"assignee_id,omitempty"`
	Statuses    []string `json:"statuses,omitempty"`
}

func CreateProjectHandler(projects repo.ProjectRepo) mcp.ToolHandlerFor[createProjectInput, *model.Project] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in createProjectInput) (*mcp.CallToolResult, *model.Project, error) {
		if in.UserID == "" || in.Name == "" {
			return nil, nil, errors.New("user_id and name are required")
		}
		p := &model.Project{
			ID:          uuid.NewString(),
			Name:        in.Name,
			Description: in.Description,
			DueDate:     in.DueDate,
			OwnerID:     in.UserID,
			AssigneeID:  in.AssigneeID,
		}
		if err := projects.Create(ctx, p, in.Statuses...); err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	}
}

// ── update_project ────────────────────────────────────────────────────────────

var UpdateProjectTool = &mcp.Tool{
	Name:        "update_project",
	Description: "Update project fields and/or add or remove custom statuses. Removing a status fails if any tasks still use it.",
}

type updateProjectInput struct {
	UserID         string   `json:"user_id"`
	ProjectID      string   `json:"project_id"`
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	DueDate        *string  `json:"due_date,omitempty"`
	AssigneeID     *string  `json:"assignee_id,omitempty"`
	AddStatuses    []string `json:"add_statuses,omitempty"`
	RemoveStatuses []string `json:"remove_statuses,omitempty"`
}

func UpdateProjectHandler(projects repo.ProjectRepo) mcp.ToolHandlerFor[updateProjectInput, *model.Project] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in updateProjectInput) (*mcp.CallToolResult, *model.Project, error) {
		if in.UserID == "" || in.ProjectID == "" {
			return nil, nil, errors.New("user_id and project_id are required")
		}
		if in.Name != nil || in.Description != nil || in.DueDate != nil || in.AssigneeID != nil {
			p, err := projects.Get(ctx, in.ProjectID)
			if err != nil {
				return nil, nil, err
			}
			if in.Name != nil {
				p.Name = *in.Name
			}
			if in.Description != nil {
				p.Description = in.Description
			}
			if in.DueDate != nil {
				p.DueDate = in.DueDate
			}
			if in.AssigneeID != nil {
				p.AssigneeID = in.AssigneeID
			}
			if err := projects.Update(ctx, p); err != nil {
				return nil, nil, err
			}
		}
		for _, s := range in.AddStatuses {
			if err := projects.AddStatus(ctx, in.ProjectID, s); err != nil {
				return nil, nil, err
			}
		}
		for _, s := range in.RemoveStatuses {
			if err := projects.DeleteStatus(ctx, in.ProjectID, s); err != nil {
				return nil, nil, err
			}
		}
		p, err := projects.Get(ctx, in.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	}
}
