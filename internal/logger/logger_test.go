package logger_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/logger"
)

func TestFromCtx_FallsBackToDefault(t *testing.T) {
	got := logger.FromCtx(context.Background())
	if got != slog.Default() {
		t.Error("expected slog.Default() when no logger in context")
	}
}

func TestIntoCtx_FromCtx_RoundTrip(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := logger.IntoCtx(context.Background(), l)
	got := logger.FromCtx(ctx)
	if got != l {
		t.Error("FromCtx did not return the logger stored by IntoCtx")
	}
}

func TestFromCtx_NilLoggerFallsBackToDefault(t *testing.T) {
	ctx := logger.IntoCtx(context.Background(), nil)
	got := logger.FromCtx(ctx)
	if got != slog.Default() {
		t.Error("expected slog.Default() when nil logger in context")
	}
}
