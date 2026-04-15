package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

type projectStore struct{ db *sql.DB }

// List returns all projects where userID is the owner or an explicit member.
func (s *projectStore) List(ctx context.Context, userID string) ([]*model.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.name, p.description, p.due_date,
		                p.owner_id, p.assignee_id, p.created_at, p.updated_at
		FROM projects p
		LEFT JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = ?
		WHERE p.owner_id = ? OR pm.user_id = ?
		ORDER BY p.created_at DESC`,
		userID, userID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Get fetches a single project by ID. Returns repo.ErrNotFound if absent.
func (s *projectStore) Get(ctx context.Context, id string) (*model.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, due_date,
		       owner_id, assignee_id, created_at, updated_at
		FROM projects WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("get project: %w", err)
		}
		return nil, repo.ErrNotFound
	}
	return scanProject(rows)
}

// Create inserts a new project and seeds the 4 default statuses in one tx.
// p.ID is always overwritten with a new UUID.
// Additional statuses are appended after the defaults (positions 4, 5, …).
// Any additional status that matches a default (case-sensitive) is silently skipped.
func (s *projectStore) Create(ctx context.Context, p *model.Project, additionalStatuses ...string) error {
	p.ID = uuid.New().String()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, due_date, owner_id, assignee_id)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.DueDate, p.OwnerID, p.AssigneeID,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}

	for i, status := range model.DefaultStatuses {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO project_statuses (project_id, status, position) VALUES (?, ?, ?)`,
			p.ID, status, i,
		)
		if err != nil {
			return fmt.Errorf("seed status %q: %w", status, err)
		}
	}

	defaults := make(map[string]bool, len(model.DefaultStatuses))
	for _, d := range model.DefaultStatuses {
		defaults[d] = true
	}
	pos := len(model.DefaultStatuses)
	for _, status := range additionalStatuses {
		if defaults[status] {
			continue
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO project_statuses (project_id, status, position) VALUES (?, ?, ?)`,
			p.ID, status, pos,
		)
		if err != nil {
			return fmt.Errorf("seed extra status %q: %w", status, err)
		}
		pos++
	}

	return tx.Commit()
}

// Update applies changes from p to the stored project.
// Name is always updated. Pointer fields (Description, DueDate, AssigneeID)
// are only updated when non-nil. updated_at is always refreshed.
func (s *projectStore) Update(ctx context.Context, p *model.Project) error {
	setClauses := []string{"name = ?", "updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')"}
	args := []any{p.Name}

	if p.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *p.Description)
	}
	if p.DueDate != nil {
		setClauses = append(setClauses, "due_date = ?")
		args = append(args, *p.DueDate)
	}
	if p.AssigneeID != nil {
		setClauses = append(setClauses, "assignee_id = ?")
		args = append(args, *p.AssigneeID)
	}

	args = append(args, p.ID)
	query := "UPDATE projects SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// Delete removes a project by ID. Cascade handles members, statuses, and tasks.
func (s *projectStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

// GetMemberRole returns the caller's effective role on a project.
// The owner always has "admin" without needing a project_members row.
// Returns repo.ErrNoAccess if the user has no membership.
func (s *projectStore) GetMemberRole(ctx context.Context, projectID, userID string) (string, error) {
	var ownerID string
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id FROM projects WHERE id = ?`, projectID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", repo.ErrNotFound
		}
		return "", fmt.Errorf("get project owner: %w", err)
	}
	if ownerID == userID {
		return model.RoleAdmin, nil
	}

	var role string
	err = s.db.QueryRowContext(ctx,
		`SELECT role FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", repo.ErrNoAccess
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}

// ListMembers returns all explicit members of a project.
func (s *projectStore) ListMembers(ctx context.Context, projectID string) ([]*model.ProjectMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT project_id, user_id, role FROM project_members WHERE project_id = ?`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []*model.ProjectMember
	for rows.Next() {
		var m model.ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Role); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, &m)
	}
	return members, rows.Err()
}

// AddMember adds a user to a project. Returns repo.ErrConflict on duplicate.
func (s *projectStore) AddMember(ctx context.Context, m *model.ProjectMember) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, ?)`,
		m.ProjectID, m.UserID, m.Role,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return repo.ErrConflict
		}
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// UpdateMemberRole changes a member's role.
func (s *projectStore) UpdateMemberRole(ctx context.Context, projectID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE project_members SET role = ? WHERE project_id = ? AND user_id = ?`,
		role, projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	return nil
}

// RemoveMember removes a user from a project.
func (s *projectStore) RemoveMember(ctx context.Context, projectID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// ListStatuses returns project statuses ordered by position.
func (s *projectStore) ListStatuses(ctx context.Context, projectID string) ([]*model.ProjectStatus, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT project_id, status, position FROM project_statuses
		 WHERE project_id = ? ORDER BY position`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list statuses: %w", err)
	}
	defer rows.Close()

	var statuses []*model.ProjectStatus
	for rows.Next() {
		var ps model.ProjectStatus
		if err := rows.Scan(&ps.ProjectID, &ps.Status, &ps.Position); err != nil {
			return nil, fmt.Errorf("scan status: %w", err)
		}
		statuses = append(statuses, &ps)
	}
	return statuses, rows.Err()
}

// AddStatus appends a new status at the end of the project's status list.
// The position is computed inside a tx to avoid races.
func (s *projectStore) AddStatus(ctx context.Context, projectID, status string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var newPos int
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position)+1, 0) FROM project_statuses WHERE project_id = ?`,
		projectID,
	).Scan(&newPos)
	if err != nil {
		return fmt.Errorf("compute position: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO project_statuses (project_id, status, position) VALUES (?, ?, ?)`,
		projectID, status, newPos,
	)
	if err != nil {
		return fmt.Errorf("insert status: %w", err)
	}

	return tx.Commit()
}

// DeleteStatus removes a status from a project.
// Returns repo.ErrConflict if any tasks currently use that status.
func (s *projectStore) DeleteStatus(ctx context.Context, projectID, status string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var count int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND status = ?`,
		projectID, status,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("count tasks: %w", err)
	}
	if count > 0 {
		return repo.ErrConflict
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM project_statuses WHERE project_id = ? AND status = ?`,
		projectID, status,
	)
	if err != nil {
		return fmt.Errorf("delete status: %w", err)
	}

	return tx.Commit()
}

// scanProject reads a project row from a *sql.Rows scanner.
func scanProject(rows *sql.Rows) (*model.Project, error) {
	var p model.Project
	err := rows.Scan(
		&p.ID, &p.Name, &p.Description, &p.DueDate,
		&p.OwnerID, &p.AssigneeID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}

// isUniqueConstraint reports whether err is a SQLite UNIQUE constraint violation.
func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// isForeignKeyConstraint reports whether err is a SQLite FOREIGN KEY constraint violation.
func isForeignKeyConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}
