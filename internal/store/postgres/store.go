package postgres

import (
	"database/sql"

	"github.com/js-beaulieu/tasks-api/internal/repo"
)

// Store holds concrete Postgres implementations of all repository interfaces.
type Store struct {
	Users    repo.UserRepo
	Projects repo.ProjectRepo
	Tasks    repo.TaskRepo
	Tags     repo.TagRepo
}

// New wires up a Store from an open *sql.DB.
func New(db *sql.DB) *Store {
	return &Store{
		Users:    &userStore{db: db},
		Projects: &projectStore{db: db},
		Tasks:    &taskStore{db: db},
		Tags:     &tagStore{db: db},
	}
}
