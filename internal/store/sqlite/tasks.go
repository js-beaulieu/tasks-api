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

type taskStore struct{ db *sql.DB }

// ListChildren returns tasks belonging to projectID with the given parentID
// (nil → top-level tasks). Optional filters may narrow the result.
func (s *taskStore) ListChildren(ctx context.Context, projectID string, parentID *string, f repo.TaskFilter) ([]*model.Task, error) {
	args := []any{}
	joins := ""

	if f.Tag != nil {
		joins = "JOIN task_tags tt ON tt.task_id = t.id AND tt.tag = ? "
		args = append(args, *f.Tag)
	}

	where := "WHERE t.project_id = ? "
	args = append(args, projectID)

	if parentID == nil {
		where += "AND t.parent_id IS NULL "
	} else {
		where += "AND t.parent_id = ? "
		args = append(args, *parentID)
	}

	if f.Status != nil {
		where += "AND t.status = ? "
		args = append(args, *f.Status)
	}
	if f.AssigneeID != nil {
		where += "AND t.assignee_id = ? "
		args = append(args, *f.AssigneeID)
	}

	query := "SELECT t.id, t.project_id, t.parent_id, t.name, t.description, " +
		"t.status, t.due_date, t.owner_id, t.assignee_id, t.position, t.created_at, t.updated_at " +
		"FROM tasks t " + joins + where + "ORDER BY t.position"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err() below captures any iteration error

	var tasks []*model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Get fetches a single task by ID. Returns repo.ErrNotFound if absent.
func (s *taskStore) Get(ctx context.Context, id string) (*model.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, parent_id, name, description,
		        status, due_date, owner_id, assignee_id, position, created_at, updated_at
		 FROM tasks WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err() below captures any iteration error

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("get task: %w", err)
		}
		return nil, repo.ErrNotFound
	}
	return scanTask(rows)
}

// Create inserts a new task within a transaction.
// The task's ID is always overwritten with a new UUID.
// Status is validated against project_statuses; position is auto-assigned.
func (s *taskStore) Create(ctx context.Context, t *model.Task) error {
	t.ID = uuid.New().String()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Validate status
	if err := validateStatus(ctx, tx, t.ProjectID, t.Status); err != nil {
		return err
	}

	// Auto-assign position
	var pos int
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) + 1
		 FROM tasks
		 WHERE project_id = ? AND parent_id IS ?`,
		t.ProjectID, t.ParentID,
	).Scan(&pos)
	if err != nil {
		return fmt.Errorf("compute position: %w", err)
	}
	t.Position = pos

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tasks
		   (id, project_id, parent_id, name, description, status, due_date,
		    owner_id, assignee_id, position)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.ParentID, t.Name, t.Description, t.Status, t.DueDate,
		t.OwnerID, t.AssigneeID, t.Position,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	return tx.Commit()
}

// Update applies all fields from t to the stored task in a single transaction.
// Handles status validation, position reordering, and cross-parent/project moves.
func (s *taskStore) Update(ctx context.Context, t *model.Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Load current state
	var cur model.Task
	var curParentID sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT project_id, parent_id, position FROM tasks WHERE id = ?`, t.ID,
	).Scan(&cur.ProjectID, &curParentID, &cur.Position)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repo.ErrNotFound
		}
		return fmt.Errorf("load current task: %w", err)
	}
	if curParentID.Valid {
		cur.ParentID = &curParentID.String
	}

	// Validate status against target project
	targetProjectID := t.ProjectID
	if targetProjectID == "" {
		targetProjectID = cur.ProjectID
	}
	if err := validateStatus(ctx, tx, targetProjectID, t.Status); err != nil {
		return err
	}

	// Determine whether this is a move
	newParentID := t.ParentID
	isMove := t.ProjectID != cur.ProjectID || !parentIDsEqual(newParentID, cur.ParentID)

	if isMove {
		// Cycle guard via recursive CTE
		if err := checkCycle(ctx, tx, t.ID, newParentID); err != nil {
			return err
		}

		// Validate new parent belongs to target project
		if newParentID != nil {
			var parentProject string
			err = tx.QueryRowContext(ctx,
				`SELECT project_id FROM tasks WHERE id = ?`, *newParentID,
			).Scan(&parentProject)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return repo.ErrNotFound
				}
				return fmt.Errorf("validate new parent: %w", err)
			}
			if parentProject != targetProjectID {
				return repo.ErrConflict
			}
		}

		// Compact old sibling group (shift down to fill gap)
		_, err = tx.ExecContext(ctx,
			`UPDATE tasks SET position = position - 1
			 WHERE project_id = ? AND parent_id IS ? AND position > ?`,
			cur.ProjectID, cur.ParentID, cur.Position,
		)
		if err != nil {
			return fmt.Errorf("compact old siblings: %w", err)
		}

		// Make room in new sibling group (shift up at target position)
		_, err = tx.ExecContext(ctx,
			`UPDATE tasks SET position = position + 1
			 WHERE project_id = ? AND parent_id IS ? AND position >= ? AND id != ?`,
			targetProjectID, newParentID, t.Position, t.ID,
		)
		if err != nil {
			return fmt.Errorf("shift new siblings: %w", err)
		}
	} else {
		// Same parent — reorder within the group
		oldPos := cur.Position
		newPos := t.Position

		if newPos < oldPos {
			// Moving up: shift others down in [newPos, oldPos)
			_, err = tx.ExecContext(ctx,
				`UPDATE tasks SET position = position + 1
				 WHERE project_id = ? AND parent_id IS ? AND position >= ? AND position < ? AND id != ?`,
				cur.ProjectID, cur.ParentID, newPos, oldPos, t.ID,
			)
		} else if newPos > oldPos {
			// Moving down: shift others up in (oldPos, newPos]
			_, err = tx.ExecContext(ctx,
				`UPDATE tasks SET position = position - 1
				 WHERE project_id = ? AND parent_id IS ? AND position > ? AND position <= ? AND id != ?`,
				cur.ProjectID, cur.ParentID, oldPos, newPos, t.ID,
			)
		}
		if err != nil {
			return fmt.Errorf("reorder siblings: %w", err)
		}
	}

	// Build parent_id value for SQL
	var sqlParentID interface{}
	if newParentID != nil {
		sqlParentID = *newParentID
	}

	setClauses := []string{
		"project_id = ?",
		"parent_id = ?",
		"name = ?",
		"status = ?",
		"position = ?",
		"updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')",
	}
	args := []any{targetProjectID, sqlParentID, t.Name, t.Status, t.Position}

	if t.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *t.Description)
	}
	if t.DueDate != nil {
		setClauses = append(setClauses, "due_date = ?")
		args = append(args, *t.DueDate)
	}
	if t.AssigneeID != nil {
		setClauses = append(setClauses, "assignee_id = ?")
		args = append(args, *t.AssigneeID)
	}

	args = append(args, t.ID)
	query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	if _, err = tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	return tx.Commit()
}

// Delete removes a task by ID. The DB CASCADE removes subtasks and tags.
func (s *taskStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// validateStatus checks that status exists in project_statuses for the given project.
// Returns repo.ErrConflict if it doesn't.
func validateStatus(ctx context.Context, tx *sql.Tx, projectID, status string) error {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_statuses WHERE project_id = ? AND status = ?`,
		projectID, status,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("validate status: %w", err)
	}
	if count == 0 {
		return repo.ErrConflict
	}
	return nil
}

// checkCycle returns repo.ErrConflict if making taskID a child of newParentID
// would create a cycle (newParentID is taskID itself or a descendant of taskID).
func checkCycle(ctx context.Context, tx *sql.Tx, taskID string, newParentID *string) error {
	if newParentID == nil {
		return nil
	}
	if *newParentID == taskID {
		return repo.ErrConflict
	}

	// Walk descendants of taskID — if newParentID appears among them, it's a cycle.
	rows, err := tx.QueryContext(ctx, `
		WITH RECURSIVE descendants(id) AS (
			SELECT id FROM tasks WHERE parent_id = ?
			UNION ALL
			SELECT t.id FROM tasks t JOIN descendants d ON t.parent_id = d.id
		)
		SELECT id FROM descendants WHERE id = ?`, taskID, *newParentID)
	if err != nil {
		return fmt.Errorf("cycle check: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err() below captures any iteration error

	if rows.Next() {
		return repo.ErrConflict
	}
	return rows.Err()
}

// parentIDsEqual compares two *string parent IDs for equality.
func parentIDsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// scanTask reads a task row from a *sql.Rows scanner.
func scanTask(rows *sql.Rows) (*model.Task, error) {
	var t model.Task
	var parentID sql.NullString
	err := rows.Scan(
		&t.ID, &t.ProjectID, &parentID, &t.Name, &t.Description,
		&t.Status, &t.DueDate, &t.OwnerID, &t.AssigneeID, &t.Position,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}
	if parentID.Valid {
		t.ParentID = &parentID.String
	}
	return &t, nil
}
