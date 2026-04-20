package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/sqlite"
)

// Task creates a task in the given project owned by ownerID with an optional parentID.
// Status defaults to "todo". Fatals the test on error.
func Task(t *testing.T, s *sqlite.Store, projectID, ownerID string, parentID *string) *model.Task {
	t.Helper()

	task := &model.Task{
		ProjectID: projectID,
		ParentID:  parentID,
		Name:      "Test Task",
		OwnerID:   ownerID,
		Status:    "todo",
	}
	if err := s.Tasks.Create(context.Background(), task); err != nil {
		t.Fatalf("seed.Task: %v", err)
	}
	return task
}
