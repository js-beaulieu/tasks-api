package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

func TestListProjectsHandler(t *testing.T) {
	t.Run("valid user_id returns project list without error", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			ListFn: func(_ context.Context, _ string) ([]*model.Project, error) {
				return []*model.Project{{ID: "p1", Name: "P1", OwnerID: "u1"}}, nil
			},
		}
		handler := ListProjectsHandler(pr)
		_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, listProjectsInput{UserID: "u1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output == nil {
			t.Fatal("expected non-nil output")
		}
	})

	t.Run("missing user_id returns error", func(t *testing.T) {
		handler := ListProjectsHandler(&mock.ProjectRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, listProjectsInput{})
		if err == nil {
			t.Fatal("expected error for missing user_id")
		}
	})
}

func TestGetProjectHandler(t *testing.T) {
	t.Run("not found returns ErrNotFound error", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, _ string) (*model.Project, error) {
				return nil, repo.ErrNotFound
			},
		}
		handler := GetProjectHandler(pr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, getProjectInput{ProjectID: "no-such"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("err = %v, want wrapping repo.ErrNotFound", err)
		}
	})

	t.Run("missing project_id returns error", func(t *testing.T) {
		handler := GetProjectHandler(&mock.ProjectRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, getProjectInput{})
		if err == nil {
			t.Fatal("expected error for missing project_id")
		}
	})
}

func TestCreateProjectHandler(t *testing.T) {
	t.Run("missing name returns error", func(t *testing.T) {
		handler := CreateProjectHandler(&mock.ProjectRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, createProjectInput{UserID: "u1"})
		if err == nil {
			t.Fatal("expected error for missing name")
		}
	})

	t.Run("missing user_id returns error", func(t *testing.T) {
		handler := CreateProjectHandler(&mock.ProjectRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, createProjectInput{Name: "P1"})
		if err == nil {
			t.Fatal("expected error for missing user_id")
		}
	})

	t.Run("extra statuses forwarded to ProjectRepo.Create", func(t *testing.T) {
		var gotStatuses []string
		pr := &mock.ProjectRepo{
			CreateFn: func(_ context.Context, _ *model.Project, additionalStatuses ...string) error {
				gotStatuses = additionalStatuses
				return nil
			},
		}
		handler := CreateProjectHandler(pr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, createProjectInput{
			UserID:   "u1",
			Name:     "My Project",
			Statuses: []string{"À faire", "En cours"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gotStatuses) != 2 || gotStatuses[0] != "À faire" || gotStatuses[1] != "En cours" {
			t.Errorf("gotStatuses = %v, want [À faire En cours]", gotStatuses)
		}
	})
}

func TestUpdateProjectHandler(t *testing.T) {
	t.Run("partial field update returns updated project", func(t *testing.T) {
		newName := "Updated"
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, id string) (*model.Project, error) {
				return &model.Project{ID: id, Name: "Original", OwnerID: "u1"}, nil
			},
			UpdateFn: func(_ context.Context, _ *model.Project) error { return nil },
		}
		handler := UpdateProjectHandler(pr)
		_, output, err := handler(context.Background(), &mcp.CallToolRequest{}, updateProjectInput{
			UserID:    "u1",
			ProjectID: "p1",
			Name:      &newName,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output == nil {
			t.Fatal("expected non-nil output")
		}
	})

	t.Run("add_statuses calls ProjectRepo.AddStatus for each", func(t *testing.T) {
		var added []string
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, id string) (*model.Project, error) {
				return &model.Project{ID: id, Name: "P", OwnerID: "u1"}, nil
			},
			AddStatusFn: func(_ context.Context, _ string, status string) error {
				added = append(added, status)
				return nil
			},
		}
		handler := UpdateProjectHandler(pr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, updateProjectInput{
			UserID:      "u1",
			ProjectID:   "p1",
			AddStatuses: []string{"review", "staging"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(added) != 2 || added[0] != "review" || added[1] != "staging" {
			t.Errorf("added = %v, want [review staging]", added)
		}
	})

	t.Run("remove_statuses with active tasks returns ErrConflict error", func(t *testing.T) {
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, id string) (*model.Project, error) {
				return &model.Project{ID: id, Name: "P", OwnerID: "u1"}, nil
			},
			DeleteStatusFn: func(_ context.Context, _, _ string) error { return repo.ErrConflict },
		}
		handler := UpdateProjectHandler(pr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, updateProjectInput{
			UserID:         "u1",
			ProjectID:      "p1",
			RemoveStatuses: []string{"todo"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, repo.ErrConflict) {
			t.Errorf("err = %v, want wrapping repo.ErrConflict", err)
		}
	})

	t.Run("remove_statuses with no active tasks calls DeleteStatus for each", func(t *testing.T) {
		var deleted []string
		pr := &mock.ProjectRepo{
			GetFn: func(_ context.Context, id string) (*model.Project, error) {
				return &model.Project{ID: id, Name: "P", OwnerID: "u1"}, nil
			},
			DeleteStatusFn: func(_ context.Context, _ string, status string) error {
				deleted = append(deleted, status)
				return nil
			},
		}
		handler := UpdateProjectHandler(pr)
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, updateProjectInput{
			UserID:         "u1",
			ProjectID:      "p1",
			RemoveStatuses: []string{"review", "staging"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deleted) != 2 || deleted[0] != "review" || deleted[1] != "staging" {
			t.Errorf("deleted = %v, want [review staging]", deleted)
		}
	})

	t.Run("missing user_id or project_id returns error", func(t *testing.T) {
		handler := UpdateProjectHandler(&mock.ProjectRepo{})
		_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, updateProjectInput{UserID: "u1"})
		if err == nil {
			t.Fatal("expected error for missing project_id")
		}
	})
}
