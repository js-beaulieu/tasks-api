package seed

import (
	"context"
	"testing"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

// User creates a user and fatals the test if it fails.
func User(t *testing.T, s *sqlite.Store, id, name, email string) *model.User {
	t.Helper()

	u, err := s.Users.Create(context.Background(), id, name, email)
	if err != nil {
		t.Fatalf("seed.User(%q): %v", id, err)
	}

	return u
}
