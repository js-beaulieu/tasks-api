package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
)

var (
	containerOnce sync.Once
	containerMu   sync.Mutex
	containerDB   *sql.DB
	containerErr  error
	containerURL  *url.URL
)

// CreateDatabaseConnection provisions a fresh Postgres database for the current test and
// returns a DSN that callers can open with the app's postgres.Open helper.
func CreateDatabaseConnection(t *testing.T) string {
	t.Helper()

	adminDB := openContainerDB(t)
	dbName := databaseName(t.Name())
	ctx := context.Background()

	containerMu.Lock()
	defer containerMu.Unlock()

	if _, err := adminDB.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(dbName)); err != nil {
		t.Fatalf("create test database %q: %v", dbName, err)
	}

	t.Cleanup(func() {
		containerMu.Lock()
		defer containerMu.Unlock()

		if _, err := adminDB.ExecContext(
			ctx,
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`,
			dbName,
		); err != nil {
			t.Fatalf("terminate connections for %q: %v", dbName, err)
		}
		if _, err := adminDB.ExecContext(ctx, `DROP DATABASE IF EXISTS `+quoteIdentifier(dbName)); err != nil {
			t.Fatalf("drop test database %q: %v", dbName, err)
		}
	})

	dsn := *containerURL
	dsn.Path = "/" + dbName
	return dsn.String()
}

// DatabaseURL preserves the shorter helper name used by store integration tests.
func DatabaseURL(t *testing.T) string {
	t.Helper()
	return CreateDatabaseConnection(t)
}

// Open opens a fresh isolated Postgres DB for the given test, runs migrations,
// and returns both the raw DB handle and the concrete store.
func Open(t *testing.T) (*sql.DB, *postgres.Store) {
	t.Helper()

	rawDB, err := postgres.Open(CreateDatabaseConnection(t))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	t.Cleanup(func() { _ = rawDB.Close() })

	return rawDB, postgres.New(rawDB)
}

func openContainerDB(t *testing.T) *sql.DB {
	t.Helper()

	containerOnce.Do(func() {
		ctx := context.Background()
		ctr, err := tcpostgres.Run(
			ctx,
			"postgres:16-alpine",
			tcpostgres.WithDatabase("postgres"),
			tcpostgres.WithUsername("postgres"),
			tcpostgres.WithPassword("postgres"),
			tcpostgres.BasicWaitStrategies(),
			tcpostgres.WithSQLDriver("pgx"),
		)
		if err != nil {
			containerErr = fmt.Errorf("start postgres container: %w", err)
			return
		}

		dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			containerErr = fmt.Errorf("postgres connection string: %w", err)
			return
		}
		containerURL, err = url.Parse(dsn)
		if err != nil {
			containerErr = fmt.Errorf("parse postgres connection string: %w", err)
			return
		}

		containerDB, err = sql.Open("pgx", dsn)
		if err != nil {
			containerErr = fmt.Errorf("open admin postgres db: %w", err)
			return
		}
		if err := containerDB.PingContext(ctx); err != nil {
			containerErr = fmt.Errorf("ping admin postgres db: %w", err)
		}
	})

	if containerErr != nil {
		t.Fatalf("test postgres setup: %v", containerErr)
	}

	return containerDB
}

func databaseName(testName string) string {
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	const (
		maxPostgresIdentifierLength = 63
		prefix                      = "test_"
		separator                   = "_"
	)
	maxNameLen := maxPostgresIdentifierLength - len(prefix) - len(separator) - len(suffix)

	var b strings.Builder
	for _, r := range strings.ToLower(testName) {
		if b.Len() >= maxNameLen {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == 0 {
		b.WriteString("case")
	}

	return prefix + b.String() + separator + suffix
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
