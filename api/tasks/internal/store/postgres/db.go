package postgres

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"

	commonpg "github.com/js-beaulieu/hs-api/libs/hs-common/postgres"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens a Postgres database at the given DSN using the shared pgx helper
// and runs all pending goose migrations before returning.
//
// DSN example:
//
//	"postgres://postgres:postgres@localhost:5432/tasks_api?sslmode=disable"
func Open(dsn string) (*sql.DB, error) {
	db, err := commonpg.Open(dsn)
	if err != nil {
		return nil, err
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// bind converts "?" placeholders to PostgreSQL positional placeholders.
func bind(query string) string { return commonpg.Bind(query) }

// isUniqueConstraint reports whether err is a Postgres unique-violation (23505).
func isUniqueConstraint(err error) bool { return commonpg.IsUniqueConstraint(err) }

// isForeignKeyConstraint reports whether err is a Postgres FK violation (23503).
func isForeignKeyConstraint(err error) bool { return commonpg.IsForeignKeyConstraint(err) }
