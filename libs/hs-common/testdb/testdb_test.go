package testdb

import (
	"strings"
	"testing"
)

func TestDatabaseName(t *testing.T) {
	name := databaseName("TestFoo/Bar")
	if !strings.HasPrefix(name, "test_") {
		t.Errorf("name = %q, expected test_ prefix", name)
	}
	if len(name) > 63 {
		t.Errorf("name length %d > 63", len(name))
	}
}

func TestQuoteIdentifier(t *testing.T) {
	if got := quoteIdentifier(`a"b`); got != `"a""b"` {
		t.Errorf("quoteIdentifier = %q, want \"a\"\"b\"", got)
	}
}
