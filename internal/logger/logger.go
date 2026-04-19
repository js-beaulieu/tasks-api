package logger

import (
	"context"
	"encoding/json"
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

// Group nests v's fields under key as a slog group, using JSON tags for field
// names. Both the JSON handler ("key": {...}) and tint (key.field=val) render
// it properly without custom wrappers.
func Group(key string, v any) slog.Attr {
	data, err := json.Marshal(v)
	if err != nil {
		return slog.Any(key, v)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return slog.Any(key, v)
	}
	attrs := make([]any, 0, len(m)*2)
	for k, val := range m {
		attrs = append(attrs, k, val)
	}
	return slog.Group(key, attrs...)
}
