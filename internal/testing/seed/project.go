package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/sqlite"
)

// Project creates a project owned by ownerID and fatals the test if it fails.
func Project(t *testing.T, s *sqlite.Store, ownerID string) *model.Project {
	t.Helper()

	p := &model.Project{Name: "Test Project", OwnerID: ownerID}
	if err := s.Projects.Create(context.Background(), p); err != nil {
		t.Fatalf("seed.Project: %v", err)
	}

	return p
}
