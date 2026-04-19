package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"

	"github.com/js-beaulieu/tasks/internal/config"
)

type ctxKey struct{}

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

func IntoCtx(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
