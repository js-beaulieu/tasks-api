// Package postgres provides reusable Postgres connection and migration helpers
// for Home Stack API services.
package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens a Postgres database at the given DSN using pgx.
//
// DSN example:
//
//	"postgres://postgres:postgres@localhost:5432/tasks_api?sslmode=disable"
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

// Bind converts positional "?" placeholders to PostgreSQL "$N" placeholders.
func Bind(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)

	index := 1
	for _, r := range query {
		if r == '?' {
			fmt.Fprintf(&b, "$%d", index)
			index++
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// IsUniqueConstraint reports whether err is a Postgres unique-violation (23505).
func IsUniqueConstraint(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsForeignKeyConstraint reports whether err is a Postgres foreign-key violation (23503).
func IsForeignKeyConstraint(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
