package mock

import (
	"context"

	"github.com/js-beaulieu/tasks/internal/model"
)

// ProjectRepo is a test double for repo.ProjectRepo.
// Set each Fn field to control what the mock returns per method.
type ProjectRepo struct {
	ListFn             func(ctx context.Context, userID string) ([]*model.Project, error)
	GetFn              func(ctx context.Context, id string) (*model.Project, error)
	CreateFn           func(ctx context.Context, p *model.Project) error
	UpdateFn           func(ctx context.Context, p *model.Project) error
	DeleteFn           func(ctx context.Context, id string) error
	GetMemberRoleFn    func(ctx context.Context, projectID, userID string) (string, error)
	ListMembersFn      func(ctx context.Context, projectID string) ([]*model.ProjectMember, error)
	AddMemberFn        func(ctx context.Context, m *model.ProjectMember) error
	UpdateMemberRoleFn func(ctx context.Context, projectID, userID, role string) error
	RemoveMemberFn     func(ctx context.Context, projectID, userID string) error
	ListStatusesFn     func(ctx context.Context, projectID string) ([]*model.ProjectStatus, error)
	AddStatusFn        func(ctx context.Context, projectID, status string) error
	DeleteStatusFn     func(ctx context.Context, projectID, status string) error
}

func (m *ProjectRepo) List(ctx context.Context, userID string) ([]*model.Project, error) {
	return m.ListFn(ctx, userID)
}

func (m *ProjectRepo) Get(ctx context.Context, id string) (*model.Project, error) {
	return m.GetFn(ctx, id)
}

func (m *ProjectRepo) Create(ctx context.Context, p *model.Project) error {
	return m.CreateFn(ctx, p)
}

func (m *ProjectRepo) Update(ctx context.Context, p *model.Project) error {
	return m.UpdateFn(ctx, p)
}

func (m *ProjectRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFn(ctx, id)
}

func (m *ProjectRepo) GetMemberRole(ctx context.Context, projectID, userID string) (string, error) {
	return m.GetMemberRoleFn(ctx, projectID, userID)
}

func (m *ProjectRepo) ListMembers(ctx context.Context, projectID string) ([]*model.ProjectMember, error) {
	return m.ListMembersFn(ctx, projectID)
}

func (m *ProjectRepo) AddMember(ctx context.Context, mem *model.ProjectMember) error {
	return m.AddMemberFn(ctx, mem)
}

func (m *ProjectRepo) UpdateMemberRole(ctx context.Context, projectID, userID, role string) error {
	return m.UpdateMemberRoleFn(ctx, projectID, userID, role)
}

func (m *ProjectRepo) RemoveMember(ctx context.Context, projectID, userID string) error {
	return m.RemoveMemberFn(ctx, projectID, userID)
}

func (m *ProjectRepo) ListStatuses(ctx context.Context, projectID string) ([]*model.ProjectStatus, error) {
	return m.ListStatusesFn(ctx, projectID)
}

func (m *ProjectRepo) AddStatus(ctx context.Context, projectID, status string) error {
	return m.AddStatusFn(ctx, projectID, status)
}

func (m *ProjectRepo) DeleteStatus(ctx context.Context, projectID, status string) error {
	return m.DeleteStatusFn(ctx, projectID, status)
}
