package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

type TaskInput struct {
	ProjectID   string
	ParentID    *string
	Name        string
	Description *string
	Status      string
	DueDate     *string
	OwnerID     string
	AssigneeID  *string
	Recurrence  *string
}

// Task creates a task in Postgres and fatals on error.
func Task(t *testing.T, s *postgres.Store, in TaskInput) *model.Task {
	t.Helper()

	if in.Name == "" {
		in.Name = "Test Task"
	}
	if in.Status == "" {
		in.Status = "todo"
	}

	task := &model.Task{
		ProjectID:   in.ProjectID,
		ParentID:    in.ParentID,
		Name:        in.Name,
		Description: in.Description,
		Status:      in.Status,
		DueDate:     in.DueDate,
		OwnerID:     in.OwnerID,
		AssigneeID:  in.AssigneeID,
		Recurrence:  in.Recurrence,
	}
	if err := s.Tasks.Create(context.Background(), task); err != nil {
		t.Fatalf("seed.Task: %v", err)
	}
	return task
}
