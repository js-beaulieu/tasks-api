package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

type ProjectInput struct {
	Name               string
	Description        *string
	DueDate            *string
	OwnerID            string
	AssigneeID         *string
	AdditionalStatuses []string
}

// Project creates a project in Postgres and fatals on error.
func Project(t *testing.T, s *postgres.Store, in ProjectInput) *model.Project {
	t.Helper()

	if in.Name == "" {
		in.Name = "Test Project"
	}

	p := &model.Project{
		Name:        in.Name,
		Description: in.Description,
		DueDate:     in.DueDate,
		OwnerID:     in.OwnerID,
		AssigneeID:  in.AssigneeID,
	}
	if err := s.Projects.Create(context.Background(), p, in.AdditionalStatuses...); err != nil {
		t.Fatalf("seed.Project: %v", err)
	}
	return p
}
