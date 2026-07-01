// Package logmiddleware provides a reusable HTTP request-logging middleware for Home Stack API services.
package logmiddleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/js-beaulieu/hs-api/libs/hs-common/logger"
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

// Options holds the narrow configuration needed by the logging middleware.
type Options struct {
	Detailed bool
}

// Middleware logs request/response metadata and injects a request-scoped logger.
func Middleware(opts Options) func(http.Handler) http.Handler {
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

			var respBuf *bytes.Buffer
			if isSSE {
				log.InfoContext(ctx, "→ SSE stream opened")
			} else {
				if opts.Detailed && r.Body != nil {
					body, _ := io.ReadAll(r.Body)
					r.Body = io.NopCloser(bytes.NewReader(body))
					log.DebugContext(ctx, "→ request", "body", string(body))
				} else {
					log.DebugContext(ctx, "→ request")
				}
			if opts.Detailed {
				respBuf = &bytes.Buffer{}
			}
			}
			rw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK, body: respBuf}
			start := time.Now()
			next.ServeHTTP(rw, r.WithContext(ctx))

			if isSSE {
				log.InfoContext(ctx, "← SSE stream closed", "duration_ms", time.Since(start).Milliseconds())
			} else {
				level := slog.LevelInfo
				if rw.status >= 500 {
					level = slog.LevelError
				} else if rw.status >= 400 {
					level = slog.LevelWarn
				}
			if opts.Detailed {
				log.Log(ctx, level, "← response",
						"status", rw.status,
						"duration_ms", time.Since(start).Milliseconds(),
						"bytes", rw.written,
						"body", respBuf.String(),
					)
				} else {
					log.Log(ctx, level, "← response",
						"status", rw.status,
						"duration_ms", time.Since(start).Milliseconds(),
						"bytes", rw.written,
					)
				}
			}
		})
	}
}
