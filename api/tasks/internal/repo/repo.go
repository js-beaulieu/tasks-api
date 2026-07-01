package repo

import (
	"context"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	repoerr "github.com/js-beaulieu/hs-api/libs/hs-common/repo"
)

var (
	ErrNotFound = repoerr.ErrNotFound
	ErrNoAccess = repoerr.ErrNoAccess
	ErrConflict = repoerr.ErrConflict
)

type UserRepo interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	ListByIDs(ctx context.Context, ids []string) ([]*model.User, error)
	Search(ctx context.Context, query string, limit int) ([]*model.User, error)
	Create(ctx context.Context, id, name, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	Delete(ctx context.Context, id string) error
}

type ProjectRepo interface {
	List(ctx context.Context, userID string) ([]*model.Project, error)
	Get(ctx context.Context, id string) (*model.Project, error)
	Create(ctx context.Context, p *model.Project, additionalStatuses ...string) error
	Update(ctx context.Context, p *model.Project) error
	Delete(ctx context.Context, id string) error
	GetMemberRole(ctx context.Context, projectID, userID string) (string, error)
	ListMembers(ctx context.Context, projectID string) ([]*model.ProjectMember, error)
	AddMember(ctx context.Context, m *model.ProjectMember) error
	UpdateMemberRole(ctx context.Context, projectID, userID, role string) error
	RemoveMember(ctx context.Context, projectID, userID string) (int, error)
	ListStatuses(ctx context.Context, projectID string) ([]*model.ProjectStatus, error)
	AddStatus(ctx context.Context, projectID, status string) error
	DeleteStatus(ctx context.Context, projectID, status string) error
}

type TaskRepo interface {
	ListChildren(ctx context.Context, projectID string, parentID *string, f TaskFilter) ([]*model.Task, error)
	Get(ctx context.Context, id string) (*model.Task, error)
	Create(ctx context.Context, t *model.Task) error
	// Update modifies a task. If the status changes to "done" and the task is
	// recurring with a due_date, it also creates the next occurrence and returns
	// its ID via nextOccurrenceID (nil when no recurrence is applicable).
	Update(ctx context.Context, t *model.Task) (*model.Task, *string, error)
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
