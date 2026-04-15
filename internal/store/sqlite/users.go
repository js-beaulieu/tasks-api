package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

type userStore struct {
	db *sql.DB
}

// GetByID fetches a user by ID. Returns repo.ErrNotFound if no row exists.
func (s *userStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// Create inserts a new user. Returns repo.ErrConflict if a user with the same
// ID already exists.
func (s *userStore) Create(ctx context.Context, id, name, email string) (*model.User, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email) VALUES (?, ?, ?)`,
		id, name, email)
	if err != nil {
		if isUniqueConstraint(err) {
			return nil, repo.ErrConflict
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return s.GetByID(ctx, id)
}

// Update replaces the name and email of an existing user.
// Returns repo.ErrNotFound if no user with that ID exists.
// Returns repo.ErrConflict if the new email is already taken.
func (s *userStore) Update(ctx context.Context, u *model.User) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET name = ?, email = ? WHERE id = ?`,
		u.Name, u.Email, u.ID)
	if err != nil {
		if isUniqueConstraint(err) {
			return repo.ErrConflict
		}
		return fmt.Errorf("update user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// Delete removes a user by ID.
// Returns repo.ErrNotFound if no user with that ID exists.
// Returns repo.ErrConflict if the user still owns projects or tasks (FK RESTRICT).
func (s *userStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		if isForeignKeyConstraint(err) {
			return repo.ErrConflict
		}
		return fmt.Errorf("delete user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}
