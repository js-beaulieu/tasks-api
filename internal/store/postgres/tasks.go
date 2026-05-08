package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/js-beaulieu/tasks-api/internal/logger"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/repo"
)

type taskStore struct{ db *sql.DB }

// ListChildren returns tasks belonging to projectID with the given parentID
// (nil → top-level tasks). Optional filters may narrow the result.
func (s *taskStore) ListChildren(ctx context.Context, projectID string, parentID *string, f repo.TaskFilter) ([]*model.Task, error) {
	logger.FromCtx(ctx).Debug("listing tasks", "project_id", projectID)
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
		"t.status, t.due_date, t.owner_id, t.assignee_id, t.position, t.recurrence, t.created_at, t.updated_at " +
		"FROM tasks t " + joins + where + "ORDER BY t.position"

	rows, err := s.db.QueryContext(ctx, bind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	logger.FromCtx(ctx).Debug("listed tasks", "project_id", projectID, "count", len(tasks))
	return tasks, rows.Err()
}

// Get fetches a single task by ID. Returns repo.ErrNotFound if absent.
func (s *taskStore) Get(ctx context.Context, id string) (*model.Task, error) {
	logger.FromCtx(ctx).Debug("getting task", "id", id)
	rows, err := s.db.QueryContext(ctx,
		bind(`SELECT id, project_id, parent_id, name, description,
		        status, due_date, owner_id, assignee_id, position, recurrence, created_at, updated_at
		 FROM tasks WHERE id = ?`), id)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("get task: %w", err)
		}
		logger.FromCtx(ctx).Debug("task not found", "id", id)
		return nil, repo.ErrNotFound
	}
	logger.FromCtx(ctx).Debug("got task", "id", id)
	return scanTask(rows)
}

// Create inserts a new task within a transaction.
// The task's ID is always overwritten with a new UUID.
// Status is validated against project_statuses; position is auto-assigned.
func (s *taskStore) Create(ctx context.Context, t *model.Task) error {
	logger.FromCtx(ctx).Debug("creating task", "project_id", t.ProjectID, "name", t.Name)
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
		bind(`SELECT COALESCE(MAX(position), -1) + 1
		 FROM tasks
		 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ?`),
		t.ProjectID, t.ParentID,
	).Scan(&pos)
	if err != nil {
		return fmt.Errorf("compute position: %w", err)
	}
	t.Position = pos

	_, err = tx.ExecContext(ctx,
		bind(`INSERT INTO tasks
		   (id, project_id, parent_id, name, description, status, due_date,
		    owner_id, assignee_id, position, recurrence)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		t.ID, t.ProjectID, t.ParentID, t.Name, t.Description, t.Status, t.DueDate,
		t.OwnerID, t.AssigneeID, t.Position, t.Recurrence,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		logger.FromCtx(ctx).Error("failed to create task", "err", err)
		return err
	}
	logger.FromCtx(ctx).Debug("created task", "id", t.ID)
	return nil
}

// Update applies all fields from t to the stored task in a single transaction.
// Handles status validation, position reordering, and cross-parent/project moves.
func (s *taskStore) Update(ctx context.Context, t *model.Task) error {
	logger.FromCtx(ctx).Debug("updating task", "id", t.ID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Load current state
	var cur model.Task
	var curParentID sql.NullString
	err = tx.QueryRowContext(ctx,
		bind(`SELECT project_id, parent_id, position FROM tasks WHERE id = ?`), t.ID,
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

	if err := updateTaskPosition(ctx, tx, t.ID, cur, targetProjectID, newParentID, t.Position, isMove); err != nil {
		return err
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
		"updated_at = NOW()",
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
	if t.Recurrence != nil {
		setClauses = append(setClauses, "recurrence = ?")
		args = append(args, *t.Recurrence)
	}

	args = append(args, t.ID)
	query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	if _, err = tx.ExecContext(ctx, bind(query), args...); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		logger.FromCtx(ctx).Error("failed to update task", "err", err)
		return err
	}
	logger.FromCtx(ctx).Debug("updated task", "id", t.ID)
	return nil
}

// CompleteTask marks the task as done and, if it is recurring with a due_date,
// creates and returns the next occurrence. All changes happen in one transaction.
func (s *taskStore) CompleteTask(ctx context.Context, id, doneStatus string) (*model.Task, *model.Task, error) {
	logger.FromCtx(ctx).Debug("completing task", "id", id, "done_status", doneStatus)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Load the task to get recurrence, due_date, project_id, etc.
	rows, err := tx.QueryContext(ctx,
		bind(`SELECT id, project_id, parent_id, name, description,
		        status, due_date, owner_id, assignee_id, position, recurrence, created_at, updated_at
		 FROM tasks WHERE id = ?`), id)
	if err != nil {
		return nil, nil, fmt.Errorf("load task: %w", err)
	}
	if !rows.Next() {
		rows.Close()
		return nil, nil, repo.ErrNotFound
	}
	task, err := scanTask(rows)
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	// Validate done_status against project_statuses.
	if err := validateStatus(ctx, tx, task.ProjectID, doneStatus); err != nil {
		return nil, nil, err
	}

	// Recurring task requires a due_date to compute next occurrence.
	if task.Recurrence != nil && task.DueDate == nil {
		return nil, nil, repo.ErrConflict
	}

	// Mark the task done.
	if _, err = tx.ExecContext(ctx,
		bind(`UPDATE tasks SET status = ?, updated_at = NOW() WHERE id = ?`),
		doneStatus, id,
	); err != nil {
		return nil, nil, fmt.Errorf("update task status: %w", err)
	}
	task.Status = doneStatus

	// Non-recurring: commit and return.
	if task.Recurrence == nil {
		if err := tx.Commit(); err != nil {
			return nil, nil, fmt.Errorf("commit: %w", err)
		}
		logger.FromCtx(ctx).Debug("completed task", "id", id)
		return task, nil, nil
	}

	// Compute next due date.
	nextDue, err := nextOccurrence(*task.DueDate, *task.Recurrence)
	if err != nil {
		return nil, nil, fmt.Errorf("compute next occurrence: %w", err)
	}

	// Determine the first status for the project (lowest position).
	var firstStatus string
	if err = tx.QueryRowContext(ctx,
		bind(`SELECT status FROM project_statuses WHERE project_id = ? ORDER BY position LIMIT 1`),
		task.ProjectID,
	).Scan(&firstStatus); err != nil {
		return nil, nil, fmt.Errorf("get first status: %w", err)
	}

	// Compute position for the new task (appended to top-level siblings).
	var pos int
	if err = tx.QueryRowContext(ctx,
		bind(`SELECT COALESCE(MAX(position), -1) + 1 FROM tasks WHERE project_id = ? AND parent_id IS NULL`),
		task.ProjectID,
	).Scan(&pos); err != nil {
		return nil, nil, fmt.Errorf("compute position: %w", err)
	}

	// Build the next occurrence task.
	newID := uuid.New().String()
	newTask := &model.Task{
		ID:          newID,
		ProjectID:   task.ProjectID,
		Name:        task.Name,
		Description: task.Description,
		Status:      firstStatus,
		DueDate:     &nextDue,
		OwnerID:     task.OwnerID,
		AssigneeID:  task.AssigneeID,
		Position:    pos,
		Recurrence:  task.Recurrence,
	}

	if _, err = tx.ExecContext(ctx,
		bind(`INSERT INTO tasks
		   (id, project_id, parent_id, name, description, status, due_date,
		    owner_id, assignee_id, position, recurrence)
		 VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`),
		newTask.ID, newTask.ProjectID, newTask.Name, newTask.Description,
		newTask.Status, newTask.DueDate, newTask.OwnerID, newTask.AssigneeID,
		newTask.Position, newTask.Recurrence,
	); err != nil {
		return nil, nil, fmt.Errorf("insert next occurrence: %w", err)
	}

	// Copy tags from the original task.
	if _, err = tx.ExecContext(ctx,
		bind(`INSERT INTO task_tags (task_id, tag)
		SELECT ?, tag FROM task_tags WHERE task_id = ?
		ON CONFLICT DO NOTHING`),
		newID, id,
	); err != nil {
		return nil, nil, fmt.Errorf("copy tags: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	// Re-fetch the new task so timestamps are populated.
	createdRows, err := s.db.QueryContext(ctx,
		bind(`SELECT id, project_id, parent_id, name, description,
		        status, due_date, owner_id, assignee_id, position, recurrence, created_at, updated_at
		 FROM tasks WHERE id = ?`), newID)
	if err != nil {
		return task, newTask, nil // best-effort if re-fetch fails
	}
	defer createdRows.Close()
	if createdRows.Next() {
		if fetched, ferr := scanTask(createdRows); ferr == nil {
			newTask = fetched
		}
	}

	logger.FromCtx(ctx).Debug("completed task", "id", id)
	return task, newTask, nil
}

// Delete removes a task by ID. The DB CASCADE removes subtasks and tags.
func (s *taskStore) Delete(ctx context.Context, id string) error {
	logger.FromCtx(ctx).Debug("deleting task", "id", id)
	_, err := s.db.ExecContext(ctx, bind(`DELETE FROM tasks WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	logger.FromCtx(ctx).Debug("deleted task", "id", id)
	return nil
}

// validateStatus checks that status exists in project_statuses for the given project.
// Returns repo.ErrConflict if it doesn't.
func validateStatus(ctx context.Context, tx *sql.Tx, projectID, status string) error {
	var count int
	err := tx.QueryRowContext(ctx,
		bind(`SELECT COUNT(*) FROM project_statuses WHERE project_id = ? AND status = ?`),
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

func updateTaskPosition(ctx context.Context, tx *sql.Tx, taskID string, cur model.Task, targetProjectID string, newParentID *string, newPosition int, isMove bool) error {
	if !isMove {
		return reorderTaskSiblings(ctx, tx, taskID, cur, newPosition)
	}
	if targetProjectID != cur.ProjectID {
		if err := ensureTaskIsLeaf(ctx, tx, taskID); err != nil {
			return err
		}
	}
	if err := checkCycle(ctx, tx, taskID, newParentID); err != nil {
		return err
	}
	if err := validateMoveParent(ctx, tx, targetProjectID, newParentID); err != nil {
		return err
	}
	if err := compactOldSiblings(ctx, tx, cur); err != nil {
		return err
	}
	return shiftNewSiblings(ctx, tx, taskID, targetProjectID, newParentID, newPosition)
}

func ensureTaskIsLeaf(ctx context.Context, tx *sql.Tx, taskID string) error {
	var hasChildren bool
	err := tx.QueryRowContext(ctx,
		bind(`SELECT EXISTS (SELECT 1 FROM tasks WHERE parent_id = ?)`),
		taskID,
	).Scan(&hasChildren)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	if hasChildren {
		return repo.ErrConflict
	}
	return nil
}

func validateMoveParent(ctx context.Context, tx *sql.Tx, targetProjectID string, newParentID *string) error {
	if newParentID == nil {
		return nil
	}
	var parentProject string
	err := tx.QueryRowContext(ctx,
		bind(`SELECT project_id FROM tasks WHERE id = ?`), *newParentID,
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
	return nil
}

func compactOldSiblings(ctx context.Context, tx *sql.Tx, cur model.Task) error {
	_, err := tx.ExecContext(ctx,
		bind(`UPDATE tasks SET position = position - 1
		 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND position > ?`),
		cur.ProjectID, cur.ParentID, cur.Position,
	)
	if err != nil {
		return fmt.Errorf("compact old siblings: %w", err)
	}
	return nil
}

func shiftNewSiblings(ctx context.Context, tx *sql.Tx, taskID, targetProjectID string, newParentID *string, newPosition int) error {
	_, err := tx.ExecContext(ctx,
		bind(`UPDATE tasks SET position = position + 1
		 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND position >= ? AND id != ?`),
		targetProjectID, newParentID, newPosition, taskID,
	)
	if err != nil {
		return fmt.Errorf("shift new siblings: %w", err)
	}
	return nil
}

func reorderTaskSiblings(ctx context.Context, tx *sql.Tx, taskID string, cur model.Task, newPosition int) error {
	oldPosition := cur.Position
	switch {
	case newPosition < oldPosition:
		_, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET position = position + 1
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND position >= ? AND position < ? AND id != ?`),
			cur.ProjectID, cur.ParentID, newPosition, oldPosition, taskID,
		)
		if err != nil {
			return fmt.Errorf("reorder siblings: %w", err)
		}
	case newPosition > oldPosition:
		_, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET position = position - 1
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND position > ? AND position <= ? AND id != ?`),
			cur.ProjectID, cur.ParentID, oldPosition, newPosition, taskID,
		)
		if err != nil {
			return fmt.Errorf("reorder siblings: %w", err)
		}
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
	rows, err := tx.QueryContext(ctx, bind(`
		WITH RECURSIVE descendants(id) AS (
			SELECT id FROM tasks WHERE parent_id = ?
			UNION ALL
			SELECT t.id FROM tasks t JOIN descendants d ON t.parent_id = d.id
		)
		SELECT id FROM descendants WHERE id = ?`), taskID, *newParentID)
	if err != nil {
		return fmt.Errorf("cycle check: %w", err)
	}
	defer rows.Close()

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
	var recurrence sql.NullString
	err := rows.Scan(
		&t.ID, &t.ProjectID, &parentID, &t.Name, &t.Description,
		&t.Status, &t.DueDate, &t.OwnerID, &t.AssigneeID, &t.Position,
		&recurrence, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}
	if parentID.Valid {
		t.ParentID = &parentID.String
	}
	if recurrence.Valid {
		t.Recurrence = &recurrence.String
	}
	return &t, nil
}
