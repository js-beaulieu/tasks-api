package tools

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/httpserver/projects"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

// ── list_tasks ────────────────────────────────────────────────────────────────

var ListTasksTool = &mcp.Tool{
	Name:        "list_tasks",
	Description: "List tasks. Exactly one of project_id or parent_id must be provided.",
}

type listTasksInput struct {
	ProjectID *string `json:"project_id,omitempty"`
	ParentID  *string `json:"parent_id,omitempty"`
	Status    *string `json:"status,omitempty"`
	Tag       *string `json:"tag,omitempty"`
}

type listTasksResult struct {
	Tasks []*model.Task `json:"tasks"`
}

func ListTasksHandler(tasks repo.TaskRepo) mcp.ToolHandlerFor[listTasksInput, *listTasksResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in listTasksInput) (*mcp.CallToolResult, *listTasksResult, error) {
		if in.ProjectID != nil && in.ParentID != nil {
			return nil, nil, errors.New("provide exactly one of project_id or parent_id, not both")
		}
		if in.ProjectID == nil && in.ParentID == nil {
			return nil, nil, errors.New("project_id or parent_id is required")
		}

		var projectID string
		var parentID *string

		if in.ProjectID != nil {
			projectID = *in.ProjectID
		} else {
			parent, err := tasks.Get(ctx, *in.ParentID)
			if err != nil {
				return nil, nil, err
			}
			projectID = parent.ProjectID
			parentID = in.ParentID
		}

		f := repo.TaskFilter{Status: in.Status, Tag: in.Tag}
		list, err := tasks.ListChildren(ctx, projectID, parentID, f)
		if err != nil {
			return nil, nil, err
		}
		return nil, &listTasksResult{Tasks: list}, nil
	}
}

// ── get_task ──────────────────────────────────────────────────────────────────

var GetTaskTool = &mcp.Tool{
	Name:        "get_task",
	Description: "Get a task by ID.",
}

type getTaskInput struct {
	TaskID string `json:"task_id"`
}

func GetTaskHandler(tasks repo.TaskRepo) mcp.ToolHandlerFor[getTaskInput, *model.Task] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in getTaskInput) (*mcp.CallToolResult, *model.Task, error) {
		if in.TaskID == "" {
			return nil, nil, errors.New("task_id is required")
		}
		t, err := tasks.Get(ctx, in.TaskID)
		if err != nil {
			return nil, nil, err
		}
		return nil, t, nil
	}
}

// ── create_task ───────────────────────────────────────────────────────────────

var CreateTaskTool = &mcp.Tool{
	Name:        "create_task",
	Description: "Create a new task in a project.",
}

type createTaskInput struct {
	UserID      string  `json:"user_id"`
	ProjectID   string  `json:"project_id"`
	ParentID    *string `json:"parent_id,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
}

func CreateTaskHandler(projectsRepo repo.ProjectRepo, tasks repo.TaskRepo) mcp.ToolHandlerFor[createTaskInput, *model.Task] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in createTaskInput) (*mcp.CallToolResult, *model.Task, error) {
		if in.UserID == "" || in.ProjectID == "" || in.Name == "" {
			return nil, nil, errors.New("user_id, project_id, and name are required")
		}
		role, err := projectsRepo.GetMemberRole(ctx, in.ProjectID, in.UserID)
		if err != nil || !projects.RequireRole(model.RoleModify, role) {
			return nil, nil, errors.New("no access")
		}
		status := in.Status
		if status == "" {
			status = "todo"
		}
		t := &model.Task{
			ID:          uuid.NewString(),
			ProjectID:   in.ProjectID,
			ParentID:    in.ParentID,
			Name:        in.Name,
			Description: in.Description,
			Status:      status,
			DueDate:     in.DueDate,
			OwnerID:     in.UserID,
			AssigneeID:  in.AssigneeID,
		}
		if err := tasks.Create(ctx, t); err != nil {
			return nil, nil, err
		}
		return nil, t, nil
	}
}

// ── update_task ───────────────────────────────────────────────────────────────

var UpdateTaskTool = &mcp.Tool{
	Name:        "update_task",
	Description: "Update task fields.",
}

type updateTaskInput struct {
	UserID      string  `json:"user_id"`
	TaskID      string  `json:"task_id"`
	ProjectID   *string `json:"project_id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	Position    *int    `json:"position,omitempty"`
}

func UpdateTaskHandler(projectsRepo repo.ProjectRepo, tasks repo.TaskRepo) mcp.ToolHandlerFor[updateTaskInput, *model.Task] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in updateTaskInput) (*mcp.CallToolResult, *model.Task, error) {
		if in.UserID == "" || in.TaskID == "" {
			return nil, nil, errors.New("user_id and task_id are required")
		}
		t, err := tasks.Get(ctx, in.TaskID)
		if err != nil {
			return nil, nil, err
		}
		role, err := projectsRepo.GetMemberRole(ctx, t.ProjectID, in.UserID)
		if err != nil || !projects.RequireRole(model.RoleModify, role) {
			return nil, nil, errors.New("no access")
		}
		if in.ProjectID != nil && *in.ProjectID != t.ProjectID {
			targetRole, err := projectsRepo.GetMemberRole(ctx, *in.ProjectID, in.UserID)
			if err != nil || !projects.RequireRole(model.RoleModify, targetRole) {
				return nil, nil, errors.New("no access to target project")
			}
		}
		if in.Name != nil {
			t.Name = *in.Name
		}
		if in.Description != nil {
			t.Description = in.Description
		}
		if in.Status != nil {
			t.Status = *in.Status
		}
		if in.DueDate != nil {
			t.DueDate = in.DueDate
		}
		if in.AssigneeID != nil {
			t.AssigneeID = in.AssigneeID
		}
		if in.Position != nil {
			t.Position = *in.Position
		}
		if in.ProjectID != nil {
			t.ProjectID = *in.ProjectID
		}
		if err := tasks.Update(ctx, t); err != nil {
			return nil, nil, err
		}
		return nil, t, nil
	}
}

// ── complete_task ─────────────────────────────────────────────────────────────

var CompleteTaskTool = &mcp.Tool{
	Name:        "complete_task",
	Description: "Mark a task as done. If the task is recurring, creates and returns the next occurrence automatically.",
}

type completeTaskInput struct {
	UserID     string `json:"user_id"`
	TaskID     string `json:"task_id"`
	DoneStatus string `json:"done_status"`
}

type completeTaskResult struct {
	Completed *model.Task `json:"completed"`
	Next      *model.Task `json:"next"`
}

func CompleteTaskHandler(projectsRepo repo.ProjectRepo, tasks repo.TaskRepo) mcp.ToolHandlerFor[completeTaskInput, *completeTaskResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in completeTaskInput) (*mcp.CallToolResult, *completeTaskResult, error) {
		if in.UserID == "" || in.TaskID == "" || in.DoneStatus == "" {
			return nil, nil, errors.New("user_id, task_id, and done_status are required")
		}
		task, err := tasks.Get(ctx, in.TaskID)
		if err != nil {
			return nil, nil, err
		}
		role, err := projectsRepo.GetMemberRole(ctx, task.ProjectID, in.UserID)
		if err != nil || !projects.RequireRole(model.RoleModify, role) {
			return nil, nil, errors.New("no access")
		}
		completed, next, err := tasks.CompleteTask(ctx, in.TaskID, in.DoneStatus)
		if err != nil {
			return nil, nil, err
		}
		return nil, &completeTaskResult{Completed: completed, Next: next}, nil
	}
}

// ── delete_task ───────────────────────────────────────────────────────────────

var DeleteTaskTool = &mcp.Tool{
	Name:        "delete_task",
	Description: "Delete a task. Requires modify role on the project.",
}

type deleteTaskInput struct {
	UserID string `json:"user_id"`
	TaskID string `json:"task_id"`
}

func DeleteTaskHandler(projectsRepo repo.ProjectRepo, tasks repo.TaskRepo) mcp.ToolHandlerFor[deleteTaskInput, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in deleteTaskInput) (*mcp.CallToolResult, any, error) {
		if in.UserID == "" || in.TaskID == "" {
			return nil, nil, errors.New("user_id and task_id are required")
		}
		t, err := tasks.Get(ctx, in.TaskID)
		if err != nil {
			return nil, nil, err
		}
		role, err := projectsRepo.GetMemberRole(ctx, t.ProjectID, in.UserID)
		if err != nil {
			return nil, nil, err
		}
		if !projects.RequireRole(model.RoleModify, role) {
			return nil, nil, errors.New("modify role required to delete a task")
		}
		if err := tasks.Delete(ctx, in.TaskID); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}
}
