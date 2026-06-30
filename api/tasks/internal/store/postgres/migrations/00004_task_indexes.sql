-- +goose Up
-- Position-critical queries now filter on status, so include it in the index
-- to cover ListChildren, compactPositions, makeRoom, countSiblings, nextPosition.
DROP INDEX IF EXISTS idx_tasks_position;
CREATE INDEX idx_tasks_position ON tasks(project_id, parent_id, status, position);

-- Support filtering tasks by assignee (ListChildren filter).
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee_id);

-- idx_tasks_position now covers queries that previously relied on this.
DROP INDEX IF EXISTS idx_tasks_project;

-- +goose Down
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
DROP INDEX IF EXISTS idx_tasks_assignee;
DROP INDEX IF EXISTS idx_tasks_position;
CREATE INDEX idx_tasks_position ON tasks(project_id, parent_id, position);