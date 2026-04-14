package mock

import (
	"context"

	"github.com/js-beaulieu/tasks/internal/model"
)

// UserRepo is a test double for repo.UserRepo.
// Set User and Err to control what the mock returns.
type UserRepo struct {
	User *model.User
	Err  error
}

func (m *UserRepo) GetByID(_ context.Context, _ string) (*model.User, error) {
	return m.User, m.Err
}

func (m *UserRepo) GetOrCreate(_ context.Context, _, _, _ string) (*model.User, error) {
	return m.User, m.Err
}
