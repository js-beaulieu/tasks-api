package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

type UserInput struct {
	ID    string
	Name  string
	Email string
}

// User creates a user and fatals the test if it fails.
func User(t *testing.T, s *postgres.Store, in UserInput) *model.User {
	t.Helper()

	if in.ID == "" {
		in.ID = "u1"
	}
	if in.Name == "" {
		in.Name = "Test User"
	}
	if in.Email == "" {
		in.Email = in.ID + "@example.com"
	}

	u, err := s.Users.Create(context.Background(), in.ID, in.Name, in.Email)
	if err != nil {
		t.Fatalf("seed.User(%q): %v", in.ID, err)
	}

	return u
}
