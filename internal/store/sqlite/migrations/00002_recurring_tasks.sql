-- +goose Up
ALTER TABLE tasks ADD COLUMN recurrence TEXT;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.
-- For dev/test purposes a full schema recreation would be required.
