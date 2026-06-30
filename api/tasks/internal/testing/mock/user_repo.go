package mock

import (
	"context"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
)

// UserRepo is a test double for repo.UserRepo.
// Set GetByIDFn / CreateFn for per-call control; fall back to User/Err when nil.
type UserRepo struct {
	User        *model.User
	Users       []*model.User
	Err         error
	GetByIDFn   func(ctx context.Context, id string) (*model.User, error)
	ListByIDsFn func(ctx context.Context, ids []string) ([]*model.User, error)
	SearchFn    func(ctx context.Context, query string, limit int) ([]*model.User, error)
	CreateFn    func(ctx context.Context, id, name, email string) (*model.User, error)
}

func (m *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return m.User, m.Err
}

func (m *UserRepo) ListByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	if m.ListByIDsFn != nil {
		return m.ListByIDsFn(ctx, ids)
	}
	return m.Users, m.Err
}

func (m *UserRepo) Search(ctx context.Context, query string, limit int) ([]*model.User, error) {
	if m.SearchFn != nil {
		return m.SearchFn(ctx, query, limit)
	}
	return m.Users, m.Err
}

func (m *UserRepo) Create(ctx context.Context, id, name, email string) (*model.User, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, id, name, email)
	}
	return m.User, m.Err
}

func (m *UserRepo) Update(_ context.Context, _ *model.User) error {
	return m.Err
}

func (m *UserRepo) Delete(_ context.Context, _ string) error {
	return m.Err
}
