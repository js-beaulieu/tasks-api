package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/config"
	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/logger"
)

func TestLogging_EchosProvidedRequestID(t *testing.T) {
	handler := middleware.Logging(config.Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "my-correlation-id")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "my-correlation-id" {
		t.Errorf("X-Request-ID = %q, want %q", got, "my-correlation-id")
	}
}

func TestLogging_GeneratesRequestIDWhenAbsent(t *testing.T) {
	handler := middleware.Logging(config.Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if id := w.Header().Get("X-Request-ID"); id == "" {
		t.Error("expected X-Request-ID header to be set when not provided")
	}
}

func TestLogging_InjectsLoggerWithRequestID(t *testing.T) {
	const suppliedID = "trace-abc"
	var loggerFromCtx interface{}

	handler := middleware.Logging(config.Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loggerFromCtx = logger.FromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", suppliedID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if loggerFromCtx == nil {
		t.Error("expected a logger to be stored in request context")
	}
}

func TestLogging_CallsNextHandler(t *testing.T) {
	called := false
	handler := middleware.Logging(config.Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/things", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called")
	}
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}
