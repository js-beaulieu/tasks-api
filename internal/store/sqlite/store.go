package sqlite

import (
	"database/sql"

	"github.com/js-beaulieu/tasks/internal/repo"
)

// Store holds concrete sqlite implementations of all repository interfaces.
type Store struct {
	Users    repo.UserRepo
	Projects repo.ProjectRepo
	Tasks    repo.TaskRepo
	Tags     repo.TagRepo
}

// New wires up a Store from an open *sql.DB.
func New(db *sql.DB) *Store {
	return &Store{
		Users: &userStore{db: db},
	}
}
