package mock

import (
	"context"

	"github.com/js-beaulieu/tasks-api/internal/model"
)

// UserRepo is a test double for repo.UserRepo.
// Set GetByIDFn / CreateFn for per-call control; fall back to User/Err when nil.
type UserRepo struct {
	User      *model.User
	Err       error
	GetByIDFn func(ctx context.Context, id string) (*model.User, error)
	CreateFn  func(ctx context.Context, id, name, email string) (*model.User, error)
	UpdateFn  func(ctx context.Context, u *model.User) error
}

func (m *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return m.User, m.Err
}

func (m *UserRepo) Create(ctx context.Context, id, name, email string) (*model.User, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, id, name, email)
	}
	return m.User, m.Err
}

func (m *UserRepo) Update(ctx context.Context, u *model.User) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, u)
	}
	return m.Err
}

func (m *UserRepo) Delete(_ context.Context, _ string) error {
	return m.Err
}
