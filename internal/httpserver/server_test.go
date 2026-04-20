package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/config"
	testdb "github.com/js-beaulieu/tasks/internal/testing/db"
)

func TestNew(t *testing.T) {
	_, store := testdb.Open(t)
	h := New(store, config.Config{})
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

func TestWellKnownHandler(t *testing.T) {
	cfg := config.Config{
		ZitadelIssuer:   "https://issuer.example.com",
		ZitadelAuthURL:  "https://issuer.example.com/oauth/v2/authorize",
		ZitadelTokenURL: "https://issuer.example.com/oauth/v2/token",
		ZitadelJWKSURL:  "https://issuer.example.com/oauth/v2/keys",
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	wellKnownHandler(cfg)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	checks := map[string]string{
		"issuer":                 cfg.ZitadelIssuer,
		"authorization_endpoint": cfg.ZitadelAuthURL,
		"token_endpoint":         cfg.ZitadelTokenURL,
		"jwks_uri":               cfg.ZitadelJWKSURL,
	}
	for key, want := range checks {
		got, ok := body[key]
		if !ok {
			t.Fatalf("missing field %q in response", key)
		}
		if got != want {
			t.Fatalf("field %q: expected %q, got %q", key, want, got)
		}
	}
}
