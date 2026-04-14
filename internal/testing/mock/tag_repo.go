package mock

import "context"

// TagRepo is a test double for repo.TagRepo.
// Set each Fn field to control what the mock returns per method.
type TagRepo struct {
	ListForTaskFn         func(ctx context.Context, taskID string) ([]string, error)
	AddFn                 func(ctx context.Context, taskID, tag string) error
	DeleteTagFn           func(ctx context.Context, taskID, tag string) error
	ListDistinctForUserFn func(ctx context.Context, userID string) ([]string, error)
}

func (m *TagRepo) ListForTask(ctx context.Context, taskID string) ([]string, error) {
	return m.ListForTaskFn(ctx, taskID)
}

func (m *TagRepo) Add(ctx context.Context, taskID, tag string) error {
	return m.AddFn(ctx, taskID, tag)
}

func (m *TagRepo) Delete(ctx context.Context, taskID, tag string) error {
	return m.DeleteTagFn(ctx, taskID, tag)
}

func (m *TagRepo) ListDistinctForUser(ctx context.Context, userID string) ([]string, error) {
	return m.ListDistinctForUserFn(ctx, userID)
}
