package mock

import (
	"context"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

// TaskRepo is a test double for repo.TaskRepo.
// Set each Fn field to control what the mock returns per method.
type TaskRepo struct {
	ListChildrenFn func(ctx context.Context, projectID string, parentID *string, f repo.TaskFilter) ([]*model.Task, error)
	GetFn          func(ctx context.Context, id string) (*model.Task, error)
	CreateFn       func(ctx context.Context, t *model.Task) error
	UpdateFn       func(ctx context.Context, t *model.Task) error
	DeleteFn       func(ctx context.Context, id string) error
	CompleteTaskFn func(ctx context.Context, id, doneStatus string) (*model.Task, *model.Task, error)
}

func (m *TaskRepo) ListChildren(ctx context.Context, projectID string, parentID *string, f repo.TaskFilter) ([]*model.Task, error) {
	return m.ListChildrenFn(ctx, projectID, parentID, f)
}

func (m *TaskRepo) Get(ctx context.Context, id string) (*model.Task, error) {
	return m.GetFn(ctx, id)
}

func (m *TaskRepo) Create(ctx context.Context, t *model.Task) error {
	return m.CreateFn(ctx, t)
}

func (m *TaskRepo) Update(ctx context.Context, t *model.Task) error {
	return m.UpdateFn(ctx, t)
}

func (m *TaskRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFn(ctx, id)
}

func (m *TaskRepo) CompleteTask(ctx context.Context, id, doneStatus string) (*model.Task, *model.Task, error) {
	return m.CompleteTaskFn(ctx, id, doneStatus)
}
