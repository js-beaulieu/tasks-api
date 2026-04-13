package repo

import (
	"context"
	"errors"

	"github.com/js-beaulieu/tasks/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrNoAccess = errors.New("no access")
var ErrConflict = errors.New("conflict")

type UserRepo interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type ProjectRepo interface {
	List(ctx context.Context, userID string) ([]*model.Project, error)
	Get(ctx context.Context, id string) (*model.Project, error)
	Create(ctx context.Context, p *model.Project) error
	Update(ctx context.Context, p *model.Project) error
	Delete(ctx context.Context, id string) error
	GetMemberRole(ctx context.Context, projectID, userID string) (string, error)
	ListMembers(ctx context.Context, projectID string) ([]*model.ProjectMember, error)
	AddMember(ctx context.Context, m *model.ProjectMember) error
	UpdateMemberRole(ctx context.Context, projectID, userID, role string) error
	RemoveMember(ctx context.Context, projectID, userID string) error
	ListStatuses(ctx context.Context, projectID string) ([]*model.ProjectStatus, error)
	AddStatus(ctx context.Context, projectID, status string) error
	DeleteStatus(ctx context.Context, projectID, status string) error
}

type TaskRepo interface {
	ListChildren(ctx context.Context, projectID string, parentID *string, f TaskFilter) ([]*model.Task, error)
	Get(ctx context.Context, id string) (*model.Task, error)
	Create(ctx context.Context, t *model.Task) error
	Update(ctx context.Context, t *model.Task) error
	Delete(ctx context.Context, id string) error
}

type TagRepo interface {
	ListForTask(ctx context.Context, taskID string) ([]string, error)
	Add(ctx context.Context, taskID, tag string) error
	Delete(ctx context.Context, taskID, tag string) error
	ListDistinctForUser(ctx context.Context, userID string) ([]string, error)
}

type TaskFilter struct {
	Status     *string
	AssigneeID *string
	Tag        *string
}
