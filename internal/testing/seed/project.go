package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

// Project creates a project in Postgres and fatals on error.
func Project(t *testing.T, s *postgres.Store, ownerID string, additionalStatuses ...string) *model.Project {
	t.Helper()

	p := &model.Project{Name: "Test Project", OwnerID: ownerID}
	if err := s.Projects.Create(context.Background(), p, additionalStatuses...); err != nil {
		t.Fatalf("seed.Project: %v", err)
	}
	return p
}
