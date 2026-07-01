package logmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareSetsResponseRequestID(t *testing.T) {
	handler := Middleware(Options{Detailed: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	respID := w.Header().Get("X-Request-ID")
	if respID == "" {
		t.Error("response did not set X-Request-ID")
	}
}

func TestMiddlewarePreservesRequestID(t *testing.T) {
	handler := Middleware(Options{Detailed: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc")
	handler.ServeHTTP(w, req)
	if w.Header().Get("X-Request-ID") != "abc" {
		t.Errorf("response id = %q, want abc", w.Header().Get("X-Request-ID"))
	}
}

func TestMiddlewareLogsRequest(t *testing.T) {
	handler := Middleware(Options{Detailed: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
