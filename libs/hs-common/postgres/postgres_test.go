package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestBind(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"SELECT * FROM t WHERE a = ? AND b = ?", "SELECT * FROM t WHERE a = $1 AND b = $2"},
		{"SELECT * FROM t", "SELECT * FROM t"},
		{"UPDATE t SET a = ?", "UPDATE t SET a = $1"},
	}
	for _, c := range cases {
		if got := Bind(c.query); got != c.want {
			t.Errorf("Bind(%q) = %q, want %q", c.query, got, c.want)
		}
	}
}

func TestIsUniqueConstraint(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if !IsUniqueConstraint(pgErr) {
		t.Error("expected unique constraint true")
	}
	if IsUniqueConstraint(errors.New("other")) {
		t.Error("expected unique constraint false for non-pg error")
	}
}

func TestIsForeignKeyConstraint(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503"}
	if !IsForeignKeyConstraint(pgErr) {
		t.Error("expected FK constraint true")
	}
}
