package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/js-beaulieu/tasks-api/internal/logger"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

type userStore struct {
	db *sql.DB
}

// GetByID fetches a user by ID. Returns repo.ErrNotFound if no row exists.
func (s *userStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	logger.FromCtx(ctx).Debug("getting user", "id", id)
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, created_at FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			logger.FromCtx(ctx).Debug("user not found", "id", id)
		}
		return nil, err
	}
	logger.FromCtx(ctx).Debug("got user", "id", id)
	return u, nil
}

// Create inserts a new user. Returns repo.ErrConflict if a user with the same
// ID already exists.
func (s *userStore) Create(ctx context.Context, id, name, email string) (*model.User, error) {
	logger.FromCtx(ctx).Debug("creating user", "id", id)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email) VALUES (?, ?, ?)`,
		id, name, email)
	if err != nil {
		if isUniqueConstraint(err) {
			return nil, repo.ErrConflict
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	logger.FromCtx(ctx).Debug("created user", "id", id)
	return s.GetByID(ctx, id)
}

// Update replaces the name and email of an existing user.
// Returns repo.ErrNotFound if no user with that ID exists.
// Returns repo.ErrConflict if the new email is already taken.
func (s *userStore) Update(ctx context.Context, u *model.User) error {
	logger.FromCtx(ctx).Debug("updating user", "id", u.ID)
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
	logger.FromCtx(ctx).Debug("updated user", "id", u.ID)
	return nil
}

// Delete removes a user by ID.
// Returns repo.ErrNotFound if no user with that ID exists.
// Returns repo.ErrConflict if the user still owns projects or tasks (FK RESTRICT).
func (s *userStore) Delete(ctx context.Context, id string) error {
	logger.FromCtx(ctx).Debug("deleting user", "id", id)
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
	logger.FromCtx(ctx).Debug("deleted user", "id", id)
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
