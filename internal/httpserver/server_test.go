package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	testdb "github.com/js-beaulieu/tasks/internal/testing/db"
)

func TestNew(t *testing.T) {
	_, store := testdb.Open(t)
	h := New(store)
	if h == nil {
		t.Fatal("expected non-nil http.Handler")
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}
