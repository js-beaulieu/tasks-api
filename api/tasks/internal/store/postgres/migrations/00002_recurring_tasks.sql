-- +goose Up
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS recurrence TEXT;

-- +goose Down
ALTER TABLE tasks DROP COLUMN IF EXISTS recurrence;
