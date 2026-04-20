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
	cfg := config.Config{ZitadelIssuer: "https://issuer.example.com"}
	h := New(store, cfg)

	t.Run("health is public", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("well-known is public", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
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

func TestWellKnownHandler(t *testing.T) {
	t.Run("unconfigured returns 503", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()
		wellKnownHandler(config.Config{})(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", w.Code)
		}
	})

	t.Run("configured returns RFC 8414 metadata", func(t *testing.T) {
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
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]any
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		stringFields := map[string]string{
			"issuer":                 cfg.ZitadelIssuer,
			"authorization_endpoint": cfg.ZitadelAuthURL,
			"token_endpoint":         cfg.ZitadelTokenURL,
			"jwks_uri":               cfg.ZitadelJWKSURL,
		}
		for key, want := range stringFields {
			got, ok := body[key]
			if !ok {
				t.Errorf("missing field %q", key)
				continue
			}
			if got != want {
				t.Errorf("field %q = %q, want %q", key, got, want)
			}
		}

		arrayFields := map[string][]string{
			"response_types_supported":         {"code"},
			"grant_types_supported":            {"authorization_code", "refresh_token"},
			"code_challenge_methods_supported": {"S256"},
		}
		for key, want := range arrayFields {
			raw, ok := body[key]
			if !ok {
				t.Errorf("missing field %q", key)
				continue
			}
			got, ok := raw.([]any)
			if !ok {
				t.Errorf("field %q is not an array", key)
				continue
			}
			if len(got) != len(want) {
				t.Errorf("field %q len = %d, want %d", key, len(got), len(want))
				continue
			}
			for i, v := range got {
				if v != want[i] {
					t.Errorf("field %q[%d] = %q, want %q", key, i, v, want[i])
				}
			}
		}
	})
}
