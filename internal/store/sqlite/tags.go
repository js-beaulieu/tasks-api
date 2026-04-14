package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type tagStore struct{ db *sql.DB }

// ListForTask returns all tags for the given task, sorted alphabetically.
func (s *tagStore) ListForTask(ctx context.Context, taskID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list tags for task: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// Add attaches a tag to a task. Idempotent (INSERT OR IGNORE).
func (s *tagStore) Add(ctx context.Context, taskID, tag string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, taskID, tag)
	if err != nil {
		return fmt.Errorf("add tag: %w", err)
	}
	return nil
}

// Delete removes a tag from a task.
func (s *tagStore) Delete(ctx context.Context, taskID, tag string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM task_tags WHERE task_id = ? AND tag = ?`, taskID, tag)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}

// ListDistinctForUser returns all distinct tags visible to userID (across owned
// and member projects), sorted alphabetically.
func (s *tagStore) ListDistinctForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT tt.tag
		FROM task_tags tt
		JOIN tasks t ON tt.task_id = t.id
		JOIN projects p ON t.project_id = p.id
		LEFT JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = ?
		WHERE p.owner_id = ? OR pm.user_id = ?
		ORDER BY tt.tag`,
		userID, userID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list distinct tags for user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}
