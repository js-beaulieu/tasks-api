package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/js-beaulieu/tasks/internal/config"
	"github.com/js-beaulieu/tasks/internal/logger"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status  int
	written int
	body    *bytes.Buffer
}

func (rw *loggingResponseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *loggingResponseWriter) Write(b []byte) (int, error) {
	if rw.body != nil {
		rw.body.Write(b)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

func (rw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func Logging(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", requestID)

			log := slog.Default().With(
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
			)
			ctx := logger.IntoCtx(r.Context(), log)

			isSSE := r.Header.Get("Accept") == "text/event-stream"

			if cfg.LogDetailed && r.Body != nil && !isSSE {
				body, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(body))
				log.DebugContext(ctx, "→ request", "body", string(body))
			} else {
				log.DebugContext(ctx, "→ request")
			}

			var respBuf *bytes.Buffer
			if cfg.LogDetailed && !isSSE {
				respBuf = &bytes.Buffer{}
			}
			rw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK, body: respBuf}
			start := time.Now()
			next.ServeHTTP(rw, r.WithContext(ctx))

			responseIsSSE := isSSE || strings.HasPrefix(rw.Header().Get("Content-Type"), "text/event-stream")
			if responseIsSSE {
				log.InfoContext(ctx, "← SSE stream closed", "duration_ms", time.Since(start).Milliseconds())
			} else {
				log.InfoContext(ctx, "← response",
					"status", rw.status,
					"duration_ms", time.Since(start).Milliseconds(),
					"bytes", rw.written,
				)
				if cfg.LogDetailed {
					log.DebugContext(ctx, "response body", "body", respBuf.String())
				}
			}
		})
	}
}
