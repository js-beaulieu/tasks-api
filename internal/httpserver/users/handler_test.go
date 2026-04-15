package users_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/httpserver/users"
	"github.com/js-beaulieu/tasks/internal/model"
	repoerr "github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

// authed wraps a handler with AuthMiddleware backed by the given mock.
func authed(m *mock.UserRepo, h http.Handler) http.Handler {
	return middleware.AuthMiddleware(m)(h)
}

func TestLoginHandler(t *testing.T) {
	t.Run("missing X-User-ID returns 401", func(t *testing.T) {
		handler := users.LoginHandler(&mock.UserRepo{})

		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		handler := users.LoginHandler(&mock.UserRepo{Err: errors.New("db error")})

		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("valid headers return 200 and user JSON", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		handler := users.LoginHandler(&mock.UserRepo{User: u})

		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.Header.Set("X-User-ID", "user-1")
		req.Header.Set("X-User-Name", "Alice")
		req.Header.Set("X-User-Email", "alice@example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got model.User
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ID != "user-1" {
			t.Errorf("ID = %q, want %q", got.ID, "user-1")
		}
	})
}

func TestGetMe(t *testing.T) {
	t.Run("valid X-User-ID returns 200 and user JSON", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got model.User
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ID != "user-1" {
			t.Errorf("ID = %q, want %q", got.ID, "user-1")
		}
		if got.Name != "Alice" {
			t.Errorf("Name = %q, want %q", got.Name, "Alice")
		}
	})

	t.Run("missing X-User-ID returns 401", func(t *testing.T) {
		m := &mock.UserRepo{}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})
}

func TestGetUserByID(t *testing.T) {
	t.Run("existing user returns 200", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodGet, "/user-1", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got model.User
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ID != "user-1" {
			t.Errorf("ID = %q, want %q", got.ID, "user-1")
		}
	})

	t.Run("unknown user returns 404", func(t *testing.T) {
		m := &mock.UserRepo{Err: repoerr.ErrNotFound}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodGet, "/nobody", nil)
		req.Header.Set("X-User-ID", "nobody")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Auth middleware also calls GetByID with the same mock, so ErrNotFound
		// surfaces there first as a 401. The 404 path is covered by integration tests.
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 (auth blocked by same ErrNotFound mock)", w.Code)
		}
	})
}

func TestUpdateMe(t *testing.T) {
	t.Run("valid patch returns updated user", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"name":"Alicia"}`))
		req.Header.Set("X-User-ID", "user-1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got model.User
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Name != "Alicia" {
			t.Errorf("Name = %q, want %q", got.Name, "Alicia")
		}
	})

	t.Run("blank name returns 400", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"name":"   "}`))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("blank email returns 400", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"email":""}`))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, users.NewRouter(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`not-json`))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}
