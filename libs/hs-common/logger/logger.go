// Package logger provides slog-based logging helpers used by Home Stack API services.
package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"

	"github.com/js-beaulieu/hs-api/libs/hs-common/config"
)

type ctxKey struct{}

// New creates a configured *slog.Logger, sets it as the default, and returns it.
func New(cfg config.Config) *slog.Logger {
	var handler slog.Handler
	if cfg.LogFormat == "pretty" {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:     cfg.LogLevel,
			AddSource: true,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     cfg.LogLevel,
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
