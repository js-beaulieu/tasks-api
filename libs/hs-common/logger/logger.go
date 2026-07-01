// Package logger provides slog-based logging helpers used by Home Stack API services.
package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

type ctxKey struct{}

// Options holds the narrow configuration needed to build a logger.
type Options struct {
	Format string
	Level  slog.Level
}

// New creates a configured *slog.Logger, sets it as the default, and returns it.
func New(opts Options) *slog.Logger {
	var handler slog.Handler
	if opts.Format == "pretty" {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:     opts.Level,
			AddSource: true,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     opts.Level,
			AddSource: true,
		})
	}
	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}

// IntoCtx stores a *slog.Logger in the context.
func IntoCtx(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromCtx retrieves the logger from the context, falling back to slog.Default().
func FromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
