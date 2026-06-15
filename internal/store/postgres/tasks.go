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
	defer func() { _ = tx.Rollback() }()

	// Validate status
	if err := validateStatus(ctx, tx, t.ProjectID, t.Status); err != nil {
		return err
	}
	if err := lockTaskSiblingLists(ctx, tx, taskSiblingList{projectID: t.ProjectID, parentID: t.ParentID}); err != nil {
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
// Handles status validation, position compaction, and cross-parent/project moves.
//
// Position strategy: always normalize first, then shift, then write, then
// compact again. This guarantees correctness regardless of whether the
// frontend sends contiguous or non-contiguous position values.
func (s *taskStore) Update(ctx context.Context, t *model.Task) error {
	logger.FromCtx(ctx).Debug("updating task", "id", t.ID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Load current state
	var cur model.Task
	var curParentID sql.NullString
	err = tx.QueryRowContext(ctx,
		bind(`SELECT project_id, parent_id, position FROM tasks WHERE id = ? FOR UPDATE`), t.ID,
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
	if err := lockTaskSiblingLists(ctx, tx,
		taskSiblingList{projectID: cur.ProjectID, parentID: cur.ParentID},
		taskSiblingList{projectID: targetProjectID, parentID: newParentID},
	); err != nil {
		return err
	}

	if isMove {
		if err := validateMove(ctx, tx, t.ID, cur, targetProjectID, newParentID); err != nil {
			return err
		}
	}

	// Normalize positions so arithmetic operates on contiguous 0-based indices.
	// For moves, compact only the target group; the old group is compacted
	// after the task is moved out (which closes the gap).
	compactProjectID := cur.ProjectID
	compactParentID := cur.ParentID
	if isMove {
		compactProjectID = targetProjectID
		compactParentID = newParentID
	}
	if err := compactPositions(ctx, tx, compactProjectID, compactParentID); err != nil {
		return err
	}

	// Re-read the task's position after compaction.
	err = tx.QueryRowContext(ctx,
		bind(`SELECT position FROM tasks WHERE id = ?`), t.ID,
	).Scan(&cur.Position)
	if err != nil {
		return fmt.Errorf("reload position: %w", err)
	}

	// Clamp requested position to valid range.
	siblingCount, err := countSiblings(ctx, tx, targetProjectID, newParentID)
	if err != nil {
		return err
	}
	maxPos := siblingCount - 1
	if isMove {
		maxPos = siblingCount // task will join this group
	}
	if t.Position > maxPos {
		t.Position = maxPos
	}
	if t.Position < 0 {
		t.Position = 0
	}

	// Shift siblings to make room at the requested position.
	// Skip when position is unchanged and this is not a move.
	positionChanged := t.Position != cur.Position
	if positionChanged || isMove {
		if err := makeRoom(ctx, tx, t.ID, cur, targetProjectID, newParentID, t.Position, isMove); err != nil {
			return err
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
	var sqlRecurrence interface{}
	if t.Recurrence != nil {
		sqlRecurrence = *t.Recurrence
	}
	setClauses = append(setClauses, "recurrence = ?")
	args = append(args, sqlRecurrence)

	args = append(args, t.ID)
	query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	if _, err = tx.ExecContext(ctx, bind(query), args...); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	// After a move, compact the old group to close the gap.
	if isMove {
		if err := compactPositions(ctx, tx, cur.ProjectID, cur.ParentID); err != nil {
			return err
		}
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
	defer func() { _ = tx.Rollback() }()

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
	if err := lockTaskSiblingLists(ctx, tx, taskSiblingList{projectID: task.ProjectID}); err != nil {
		return nil, nil, err
	}
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
// After deletion, compacts positions in the former sibling group.
func (s *taskStore) Delete(ctx context.Context, id string) error {
	logger.FromCtx(ctx).Debug("deleting task", "id", id)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var projectID, parentID sql.NullString
	err = tx.QueryRowContext(ctx,
		bind(`SELECT project_id, parent_id FROM tasks WHERE id = ?`), id,
	).Scan(&projectID, &parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repo.ErrNotFound
		}
		return fmt.Errorf("load task for delete: %w", err)
	}

	if _, err = tx.ExecContext(ctx, bind(`DELETE FROM tasks WHERE id = ?`), id); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	var pid *string
	if parentID.Valid {
		pid = &parentID.String
	}
	if err := compactPositions(ctx, tx, projectID.String, pid); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
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

func validateMove(ctx context.Context, tx *sql.Tx, taskID string, cur model.Task, targetProjectID string, newParentID *string) error {
	if targetProjectID != cur.ProjectID {
		if err := ensureTaskIsLeaf(ctx, tx, taskID); err != nil {
			return err
		}
	}
	if err := checkCycle(ctx, tx, taskID, newParentID); err != nil {
		return err
	}
	return validateMoveParent(ctx, tx, targetProjectID, newParentID)
}

func makeRoom(ctx context.Context, tx *sql.Tx, taskID string, cur model.Task, targetProjectID string, newParentID *string, newPosition int, isMove bool) error {
	if isMove || newPosition < cur.Position {
		_, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET position = position + 1
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND position >= ? AND id != ?`),
			targetProjectID, newParentID, newPosition, taskID,
		)
		if err != nil {
			return fmt.Errorf("shift siblings right: %w", err)
		}
	} else if newPosition > cur.Position {
		_, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET position = position - 1
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND position > ? AND position <= ? AND id != ?`),
			targetProjectID, newParentID, cur.Position, newPosition, taskID,
		)
		if err != nil {
			return fmt.Errorf("shift siblings left: %w", err)
		}
	}
	return nil
}

func countSiblings(ctx context.Context, tx *sql.Tx, projectID string, parentID *string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		bind(`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ?`),
		projectID, parentID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count siblings: %w", err)
	}
	return count, nil
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
		bind(`SELECT project_id FROM tasks WHERE id = ? FOR UPDATE`), *newParentID,
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

func compactPositions(ctx context.Context, tx *sql.Tx, projectID string, parentID *string) error {
	_, err := tx.ExecContext(ctx,
		bind(`UPDATE tasks SET position = sub.rn
		 FROM (
			 SELECT id, ROW_NUMBER() OVER (ORDER BY position) - 1 AS rn
			 FROM tasks
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ?
		 ) sub
		 WHERE tasks.id = sub.id AND tasks.position != sub.rn`),
		projectID, parentID,
	)
	if err != nil {
		return fmt.Errorf("compact positions: %w", err)
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
