// Package db provides integration-test helpers for the tasks API.
package db

import (
	"database/sql"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/store/postgres"
	"github.com/js-beaulieu/hs-api/libs/hs-common/testdb"
)

// PGConnectionString provisions a fresh test database and returns its DSN.
func PGConnectionString(t *testing.T) string {
	return testdb.PGConnectionString(t)
}

// Open opens a fresh isolated Postgres DB for the given test, runs migrations,
// and returns both the raw DB handle and the concrete store.
func Open(t *testing.T) (*sql.DB, *postgres.Store) {
	t.Helper()

	rawDB, err := postgres.Open(testdb.PGConnectionString(t))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	t.Cleanup(func() { _ = rawDB.Close() })

	return rawDB, postgres.New(rawDB)
}
