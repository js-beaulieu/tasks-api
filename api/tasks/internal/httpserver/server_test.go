package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/store/postgres"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/mock"
	"github.com/js-beaulieu/hs-api/libs/hs-common/config"
)

func TestNew(t *testing.T) {
	store := &postgres.Store{Users: &mock.UserRepo{}}
	h := New(store, config.Config{})

	t.Run("health is public", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("openapi is public", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"openapi"`) {
			t.Fatalf("body = %q, want OpenAPI document", w.Body.String())
		}
	})

	t.Run("docs use configured OpenAPI server path", func(t *testing.T) {
		h := New(store, config.Config{OpenAPIServerURL: "/tasks"})
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if !strings.Contains(w.Body.String(), `apiDescriptionUrl="/tasks/openapi.yaml"`) {
			t.Fatalf("body = %q, want prefixed OpenAPI URL", w.Body.String())
		}
	})
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
