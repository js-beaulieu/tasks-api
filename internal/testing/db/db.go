package db

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

// Open opens a fresh isolated in-memory SQLite DB for the given test and
// registers cleanup. The DB name is derived from t.Name() so each test
// gets its own independent schema and data.
func Open(t *testing.T) (*sql.DB, *sqlite.Store) {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_", ":", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(ON)", name)

	rawDB, err := sqlite.Open(dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	t.Cleanup(func() { rawDB.Close() })

	return rawDB, sqlite.New(rawDB)
}
