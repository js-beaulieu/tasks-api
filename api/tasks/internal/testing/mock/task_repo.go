package mock

import (
	"context"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
)

// TaskRepo is a test double for repo.TaskRepo.
// Set each Fn field to control what the mock returns per method.
type TaskRepo struct {
	ListChildrenFn func(ctx context.Context, projectID string, parentID *string, f repo.TaskFilter) ([]*model.Task, error)
	GetFn          func(ctx context.Context, id string) (*model.Task, error)
	CreateFn       func(ctx context.Context, t *model.Task) error
	UpdateFn       func(ctx context.Context, t *model.Task) (*model.Task, *string, error)
	DeleteFn       func(ctx context.Context, id string) error
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

func (m *TaskRepo) Update(ctx context.Context, t *model.Task) (*model.Task, *string, error) {
	return m.UpdateFn(ctx, t)
}

func (m *TaskRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFn(ctx, id)
}
