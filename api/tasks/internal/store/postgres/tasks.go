package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
	"github.com/js-beaulieu/hs-api/libs/hs-common/logger"
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
	if err := lockTaskSiblingLists(ctx, tx, taskSiblingList{projectID: t.ProjectID, parentID: t.ParentID, status: t.Status}); err != nil {
		return err
	}

	// Auto-assign position within the task's status group
	t.Position, err = nextPosition(ctx, tx, t.ProjectID, t.ParentID, t.Status)
	if err != nil {
		return err
	}

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
func (s *taskStore) Update(ctx context.Context, t *model.Task) (*model.Task, *string, error) {
	logger.FromCtx(ctx).Debug("updating task", "id", t.ID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Load current state
	var cur model.Task
	var curParentID sql.NullString
	var curStatus string
	err = tx.QueryRowContext(ctx,
		bind(`SELECT project_id, parent_id, status, position FROM tasks WHERE id = ? FOR UPDATE`), t.ID,
	).Scan(&cur.ProjectID, &curParentID, &curStatus, &cur.Position)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, repo.ErrNotFound
		}
		return nil, nil, fmt.Errorf("load current task: %w", err)
	}
	if curParentID.Valid {
		cur.ParentID = &curParentID.String
	}

	// Validate status against target project
	targetProjectID := t.ProjectID
	if targetProjectID == "" {
		targetProjectID = cur.ProjectID
	}

	// Determine target status (defaults to current if not changed)
	targetStatus := t.Status
	if targetStatus == "" {
		targetStatus = curStatus
	}

	// Determine whether this is a move (project or parent changed) or a reorder within status
	newParentID := t.ParentID
	crossProjectMove := targetProjectID != cur.ProjectID
	var moveCtx *projectMoveContext
	if crossProjectMove {
		newParentID = nil
		moveCtx, err = loadProjectMoveContext(ctx, tx, targetProjectID)
		if err != nil {
			return nil, nil, err
		}
		targetStatus = moveCtx.resolveStatus(targetStatus)
		t.AssigneeID = moveCtx.resolveAssignee(t.AssigneeID)
		t.ParentID = nil
	}
	if err := validateStatus(ctx, tx, targetProjectID, targetStatus); err != nil {
		return nil, nil, err
	}
	t.Status = targetStatus
	t.ProjectID = targetProjectID
	isMove := t.ProjectID != cur.ProjectID || !parentIDsEqual(newParentID, cur.ParentID)
	statusChanged := targetStatus != curStatus

	// Lock the relevant sibling lists: current group and (if different) the target group
	if err := lockTaskSiblingLists(ctx, tx,
		taskSiblingList{projectID: cur.ProjectID, parentID: cur.ParentID, status: curStatus},
		taskSiblingList{projectID: targetProjectID, parentID: newParentID, status: targetStatus},
	); err != nil {
		return nil, nil, err
	}

	if isMove {
		if err := validateMove(ctx, tx, t.ID, cur, targetProjectID, newParentID); err != nil {
			return nil, nil, err
		}
	}

	// Normalize positions so arithmetic operates on contiguous 0-based indices.
	// Always compact the target status group first.
	if err := compactPositions(ctx, tx, targetProjectID, newParentID, targetStatus); err != nil {
		return nil, nil, err
	}
	// If moving between groups, also compact the source group before the task leaves.
	if isMove || statusChanged {
		if err := compactPositions(ctx, tx, cur.ProjectID, cur.ParentID, curStatus); err != nil {
			return nil, nil, err
		}
	}

	// Re-read the task's position after compaction.
	err = tx.QueryRowContext(ctx,
		bind(`SELECT position FROM tasks WHERE id = ?`), t.ID,
	).Scan(&cur.Position)
	if err != nil {
		return nil, nil, fmt.Errorf("reload position: %w", err)
	}

	siblingCount, err := countSiblings(ctx, tx, targetProjectID, newParentID, targetStatus)
	if err != nil {
		return nil, nil, err
	}
	maxPos := siblingCount - 1
	if isMove || statusChanged {
		maxPos = siblingCount // task will join this group
	}
	if t.Position > maxPos {
		t.Position = maxPos
	}
	if t.Position < 0 {
		t.Position = 0
	}

	// Shift siblings to make room at the requested position.
	// For status changes, always shift (treat like a move into the new group).
	positionChanged := t.Position != cur.Position
	if positionChanged || isMove || statusChanged {
		if err := makeRoom(ctx, tx, t.ID, cur, targetProjectID, newParentID, targetStatus, t.Position, isMove || statusChanged); err != nil {
			return nil, nil, err
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
		return nil, nil, fmt.Errorf("update task: %w", err)
	}

	if crossProjectMove {
		if err := moveDescendantsToProject(ctx, tx, t.ID, targetProjectID, *moveCtx); err != nil {
			return nil, nil, err
		}
	}

	// After a move or status change, compact the old group to close the gap.
	if isMove || statusChanged {
		if err := compactPositions(ctx, tx, cur.ProjectID, cur.ParentID, curStatus); err != nil {
			return nil, nil, err
		}
	}

	// If the task was just moved to "done" status and is recurring with a due_date,
	// spawn the next occurrence and return its ID.
	var nextOccurrenceID *string
	if statusChanged && targetStatus == "done" && t.Recurrence != nil && *t.Recurrence != "" {
		if t.DueDate == nil {
			if err := tx.Rollback(); err != nil {
				return nil, nil, fmt.Errorf("rollback: %w", err)
			}
			return nil, nil, repo.ErrConflict
		}

		nextDue, err := nextOccurrence(*t.DueDate, *t.Recurrence)
		if err != nil {
			return nil, nil, fmt.Errorf("compute next occurrence: %w", err)
		}

		// Determine the first status for the project (lowest position).
		var firstStatus string
		if err = tx.QueryRowContext(ctx,
			bind(`SELECT status FROM project_statuses WHERE project_id = ? ORDER BY position LIMIT 1`),
			t.ProjectID,
		).Scan(&firstStatus); err != nil {
			return nil, nil, fmt.Errorf("get first status: %w", err)
		}

		// Compute position for the new task (appended to siblings in the first status).
		if err := lockTaskSiblingLists(ctx, tx, taskSiblingList{projectID: t.ProjectID, status: firstStatus}); err != nil {
			return nil, nil, err
		}
		pos, err := nextPosition(ctx, tx, t.ProjectID, nil, firstStatus)
		if err != nil {
			return nil, nil, err
		}

		// Build the next occurrence task.
		newID := uuid.New().String()
		nextTask := &model.Task{
			ID:          newID,
			ProjectID:   t.ProjectID,
			Name:        t.Name,
			Description: t.Description,
			Status:      firstStatus,
			DueDate:     &nextDue,
			OwnerID:     t.OwnerID,
			AssigneeID:  t.AssigneeID,
			Position:    pos,
			Recurrence:  t.Recurrence,
		}

		if _, err = tx.ExecContext(ctx,
			bind(`INSERT INTO tasks
			   (id, project_id, parent_id, name, description, status, due_date,
			    owner_id, assignee_id, position, recurrence)
			 VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`),
			nextTask.ID, nextTask.ProjectID, nextTask.Name, nextTask.Description,
			nextTask.Status, nextTask.DueDate, nextTask.OwnerID, nextTask.AssigneeID,
			nextTask.Position, nextTask.Recurrence,
		); err != nil {
			return nil, nil, fmt.Errorf("insert next occurrence: %w", err)
		}

		// Copy tags from the original task.
		if _, err = tx.ExecContext(ctx,
			bind(`INSERT INTO task_tags (task_id, tag)
			SELECT ?, tag FROM task_tags WHERE task_id = ?
			ON CONFLICT DO NOTHING`),
			newID, t.ID,
		); err != nil {
			return nil, nil, fmt.Errorf("copy tags: %w", err)
		}

		nextOccurrenceID = &newID
	}

	if err := tx.Commit(); err != nil {
		logger.FromCtx(ctx).Error("failed to update task", "err", err)
		return nil, nil, err
	}

	logger.FromCtx(ctx).Debug("updated task", "id", t.ID)
	return t, nextOccurrenceID, nil
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
	var taskStatus string
	err = tx.QueryRowContext(ctx,
		bind(`SELECT project_id, parent_id, status FROM tasks WHERE id = ?`), id,
	).Scan(&projectID, &parentID, &taskStatus)
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
	if err := compactPositions(ctx, tx, projectID.String, pid, taskStatus); err != nil {
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
	if err := checkCycle(ctx, tx, taskID, newParentID); err != nil {
		return err
	}
	return validateMoveParent(ctx, tx, targetProjectID, newParentID)
}

type projectMoveContext struct {
	firstStatus string
	ownerID     string
	statuses    map[string]struct{}
	members     map[string]struct{}
}

func (c projectMoveContext) resolveStatus(status string) string {
	if _, ok := c.statuses[status]; ok {
		return status
	}
	return c.firstStatus
}

func (c projectMoveContext) resolveAssignee(assigneeID *string) *string {
	if assigneeID != nil {
		if _, ok := c.members[*assigneeID]; ok {
			return assigneeID
		}
	}
	ownerID := c.ownerID
	return &ownerID
}

func loadProjectMoveContext(ctx context.Context, tx *sql.Tx, projectID string) (*projectMoveContext, error) {
	ctxData := &projectMoveContext{
		statuses: map[string]struct{}{},
		members:  map[string]struct{}{},
	}

	rows, err := tx.QueryContext(ctx,
		bind(`SELECT status FROM project_statuses WHERE project_id = ? ORDER BY position`),
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("load target statuses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return nil, fmt.Errorf("scan target status: %w", err)
		}
		if ctxData.firstStatus == "" {
			ctxData.firstStatus = status
		}
		ctxData.statuses[status] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate target statuses: %w", err)
	}
	if ctxData.firstStatus == "" {
		return nil, repo.ErrConflict
	}

	if err := tx.QueryRowContext(ctx,
		bind(`SELECT owner_id FROM projects WHERE id = ?`),
		projectID,
	).Scan(&ctxData.ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrNotFound
		}
		return nil, fmt.Errorf("load target owner: %w", err)
	}
	ctxData.members[ctxData.ownerID] = struct{}{}

	memberRows, err := tx.QueryContext(ctx,
		bind(`SELECT user_id FROM project_members WHERE project_id = ?`),
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("load target members: %w", err)
	}
	defer memberRows.Close()

	for memberRows.Next() {
		var userID string
		if err := memberRows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan target member: %w", err)
		}
		ctxData.members[userID] = struct{}{}
	}
	if err := memberRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate target members: %w", err)
	}

	return ctxData, nil
}

func moveDescendantsToProject(ctx context.Context, tx *sql.Tx, taskID, targetProjectID string, moveCtx projectMoveContext) error {
	type siblingGroup struct {
		parentID string
		status   string
	}
	type descendantTask struct {
		id         string
		parentID   string
		status     string
		assigneeID *string
	}

	rows, err := tx.QueryContext(ctx, bind(`
		WITH RECURSIVE descendants(id, parent_id, status, assignee_id) AS (
			SELECT id, parent_id, status, assignee_id FROM tasks WHERE parent_id = ?
			UNION ALL
			SELECT t.id, t.parent_id, t.status, t.assignee_id
			FROM tasks t
			JOIN descendants d ON t.parent_id = d.id
		)
		SELECT id, parent_id, status, assignee_id FROM descendants`), taskID)
	if err != nil {
		return fmt.Errorf("load descendants: %w", err)
	}
	defer rows.Close()

	var descendants []descendantTask
	seenGroups := map[siblingGroup]struct{}{}
	var groups []siblingGroup
	for rows.Next() {
		var descendantID string
		var parentID sql.NullString
		var status string
		var assigneeID sql.NullString
		if err := rows.Scan(&descendantID, &parentID, &status, &assigneeID); err != nil {
			return fmt.Errorf("scan descendant: %w", err)
		}
		var currentAssignee *string
		if assigneeID.Valid {
			currentAssignee = &assigneeID.String
		}
		descendants = append(descendants, descendantTask{
			id:         descendantID,
			parentID:   parentID.String,
			status:     status,
			assigneeID: currentAssignee,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate descendants: %w", err)
	}

	for _, descendant := range descendants {
		resolvedStatus := moveCtx.resolveStatus(descendant.status)
		resolvedAssignee := moveCtx.resolveAssignee(descendant.assigneeID)
		var sqlAssignee any
		if resolvedAssignee != nil {
			sqlAssignee = *resolvedAssignee
		}
		if _, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET project_id = ?, status = ?, assignee_id = ?, updated_at = NOW() WHERE id = ?`),
			targetProjectID, resolvedStatus, sqlAssignee, descendant.id,
		); err != nil {
			return fmt.Errorf("move descendant task: %w", err)
		}

		group := siblingGroup{parentID: descendant.parentID, status: resolvedStatus}
		if _, ok := seenGroups[group]; !ok {
			seenGroups[group] = struct{}{}
			groups = append(groups, group)
		}
	}

	for _, group := range groups {
		parentID := group.parentID
		if err := compactPositions(ctx, tx, targetProjectID, &parentID, group.status); err != nil {
			return err
		}
	}

	return nil
}

func makeRoom(ctx context.Context, tx *sql.Tx, taskID string, cur model.Task, targetProjectID string, newParentID *string, newStatus string, newPosition int, isMove bool) error {
	shiftRight := newPosition < cur.Position || isMove
	shiftLeft := newPosition > cur.Position && !isMove

	switch {
	case shiftRight:
		_, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET position = position + 1
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND status = ? AND position >= ? AND id != ?`),
			targetProjectID, newParentID, newStatus, newPosition, taskID,
		)
		if err != nil {
			return fmt.Errorf("shift siblings right: %w", err)
		}
	case shiftLeft:
		_, err := tx.ExecContext(ctx,
			bind(`UPDATE tasks SET position = position - 1
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND status = ? AND position > ? AND position <= ? AND id != ?`),
			targetProjectID, newParentID, newStatus, cur.Position, newPosition, taskID,
		)
		if err != nil {
			return fmt.Errorf("shift siblings left: %w", err)
		}
	}
	return nil
}

func nextPosition(ctx context.Context, tx *sql.Tx, projectID string, parentID *string, status string) (int, error) {
	var pos int
	err := tx.QueryRowContext(ctx,
		bind(`SELECT COALESCE(MAX(position), -1) + 1 FROM tasks WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND status = ?`),
		projectID, parentID, status,
	).Scan(&pos)
	if err != nil {
		return 0, fmt.Errorf("compute next position: %w", err)
	}
	return pos, nil
}

func countSiblings(ctx context.Context, tx *sql.Tx, projectID string, parentID *string, status string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		bind(`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND status = ?`),
		projectID, parentID, status,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count siblings: %w", err)
	}
	return count, nil
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

func compactPositions(ctx context.Context, tx *sql.Tx, projectID string, parentID *string, status string) error {
	_, err := tx.ExecContext(ctx,
		bind(`UPDATE tasks SET position = sub.rn
		 FROM (
			 SELECT id, ROW_NUMBER() OVER (ORDER BY position) - 1 AS rn
			 FROM tasks
			 WHERE project_id = ? AND parent_id IS NOT DISTINCT FROM ? AND status = ?
		 ) sub
		 WHERE tasks.id = sub.id AND tasks.position != sub.rn`),
		projectID, parentID, status,
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
