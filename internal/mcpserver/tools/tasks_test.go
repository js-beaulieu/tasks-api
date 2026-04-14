package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

func strPtr(s string) *string { return &s }

func TestListTasksHandler(t *testing.T) {
	t.Run("both project_id and parent_id returns error", func(t *testing.T) {
		handler := ListTasksHandler(&mock.TaskRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, listTasksInput{
			ProjectID: strPtr("p1"),
			ParentID:  strPtr("t1"),
		})
		if err == nil {
			t.Fatal("expected error when both project_id and parent_id provided")
		}
	})

	t.Run("neither project_id nor parent_id returns error", func(t *testing.T) {
		handler := ListTasksHandler(&mock.TaskRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, listTasksInput{})
		if err == nil {
			t.Fatal("expected error when neither project_id nor parent_id provided")
		}
	})

	t.Run("only project_id returns task list without error", func(t *testing.T) {
		tr := &mock.TaskRepo{
			ListChildrenFn: func(_ context.Context, _ string, _ *string, _ repo.TaskFilter) ([]*model.Task, error) {
				return []*model.Task{{ID: "t1", Name: "T1", ProjectID: "p1", OwnerID: "u1", Status: "todo"}}, nil
			},
		}
		handler := ListTasksHandler(tr)
		_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, listTasksInput{
			ProjectID: strPtr("p1"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output == nil {
			t.Fatal("expected non-nil output")
		}
	})
}

func TestGetTaskHandler(t *testing.T) {
	t.Run("not found returns error", func(t *testing.T) {
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return nil, repo.ErrNotFound
			},
		}
		handler := GetTaskHandler(tr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, getTaskInput{TaskID: "no-such"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing task_id returns error", func(t *testing.T) {
		handler := GetTaskHandler(&mock.TaskRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, getTaskInput{})
		if err == nil {
			t.Fatal("expected error for missing task_id")
		}
	})
}

func TestCreateTaskHandler(t *testing.T) {
	t.Run("valid input creates task and returns it", func(t *testing.T) {
		tr := &mock.TaskRepo{
			CreateFn: func(_ context.Context, _ *model.Task) error { return nil },
		}
		handler := CreateTaskHandler(tr)
		_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, createTaskInput{
			UserID:    "u1",
			ProjectID: "p1",
			Name:      "My Task",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output == nil {
			t.Fatal("expected non-nil output")
		}
	})

	t.Run("missing required fields returns error", func(t *testing.T) {
		handler := CreateTaskHandler(&mock.TaskRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, createTaskInput{UserID: "u1"})
		if err == nil {
			t.Fatal("expected error for missing project_id/name")
		}
	})
}

func TestUpdateTaskHandler(t *testing.T) {
	t.Run("position change calls TaskRepo.Update", func(t *testing.T) {
		updateCalled := false
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return &model.Task{ID: "t1", ProjectID: "p1", Name: "T", Status: "todo", OwnerID: "u1"}, nil
			},
			UpdateFn: func(_ context.Context, _ *model.Task) error {
				updateCalled = true
				return nil
			},
		}
		pos := 3
		handler := UpdateTaskHandler(tr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, updateTaskInput{
			TaskID:   "t1",
			Position: &pos,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updateCalled {
			t.Error("expected TaskRepo.Update to be called")
		}
	})

	t.Run("missing task_id returns error", func(t *testing.T) {
		handler := UpdateTaskHandler(&mock.TaskRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, updateTaskInput{})
		if err == nil {
			t.Fatal("expected error for missing task_id")
		}
	})
}

func TestDeleteTaskHandler(t *testing.T) {
	t.Run("caller without modify role returns error", func(t *testing.T) {
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return &model.Task{ID: "t1", ProjectID: "p1", Name: "T", Status: "todo", OwnerID: "u2"}, nil
			},
		}
		pr := &mock.ProjectRepo{
			GetMemberRoleFn: func(_ context.Context, _, _ string) (string, error) {
				return "", repo.ErrNoAccess
			},
		}
		handler := DeleteTaskHandler(pr, tr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, deleteTaskInput{
			UserID: "u1",
			TaskID: "t1",
		})
		if err == nil {
			t.Fatal("expected error for insufficient role")
		}
	})

	t.Run("modify role allows delete", func(t *testing.T) {
		deleteCalled := false
		tr := &mock.TaskRepo{
			GetFn: func(_ context.Context, _ string) (*model.Task, error) {
				return &model.Task{ID: "t1", ProjectID: "p1", Name: "T", Status: "todo", OwnerID: "u1"}, nil
			},
			DeleteFn: func(_ context.Context, _ string) error {
				deleteCalled = true
				return nil
			},
		}
		pr := &mock.ProjectRepo{
			GetMemberRoleFn: func(_ context.Context, _, _ string) (string, error) {
				return model.RoleModify, nil
			},
		}
		handler := DeleteTaskHandler(pr, tr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, deleteTaskInput{
			UserID: "u1",
			TaskID: "t1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleteCalled {
			t.Error("expected TaskRepo.Delete to be called")
		}
	})

	t.Run("missing user_id or task_id returns error", func(t *testing.T) {
		handler := DeleteTaskHandler(&mock.ProjectRepo{}, &mock.TaskRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, deleteTaskInput{UserID: "u1"})
		if err == nil {
			t.Fatal("expected error for missing task_id")
		}
	})
}
