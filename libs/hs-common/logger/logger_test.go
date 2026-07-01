package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestNew(t *testing.T) {
	l := New(Options{Format: "json", Level: slog.LevelInfo})
	if l == nil {
		t.Fatal("logger is nil")
	}
	if slog.Default() != l {
		t.Error("New did not set default logger")
	}
}

func TestCtx(t *testing.T) {
	l := New(Options{Format: "pretty", Level: slog.LevelDebug})
	ctx := IntoCtx(context.Background(), l)
	if got := FromCtx(ctx); got != l {
		t.Error("FromCtx did not return injected logger")
	}
	if got := FromCtx(context.Background()); got != slog.Default() {
		t.Error("FromCtx should fall back to default logger")
	}
}
