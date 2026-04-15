package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks/internal/model"
	repoerr "github.com/js-beaulieu/tasks/internal/repo"
	"github.com/js-beaulieu/tasks/internal/testing/mock"
)

func TestAuthMiddleware(t *testing.T) {
	okNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("missing X-User-ID returns 401", func(t *testing.T) {
		handler := middleware.AuthMiddleware(&mock.UserRepo{})(okNext)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("unknown user returns 401", func(t *testing.T) {
		repo := &mock.UserRepo{Err: repoerr.ErrNotFound}
		handler := middleware.AuthMiddleware(repo)(okNext)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	t.Run("lookup error returns 500", func(t *testing.T) {
		repo := &mock.UserRepo{Err: errors.New("db error")}
		handler := middleware.AuthMiddleware(repo)(okNext)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("valid headers inject user into context and call next", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		repo := &mock.UserRepo{User: u}

		var captured *model.User
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = middleware.UserFromCtx(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := middleware.AuthMiddleware(repo)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-User-ID", "user-1")
		req.Header.Set("X-User-Name", "Alice")
		req.Header.Set("X-User-Email", "alice@example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if captured == nil {
			t.Fatal("user was not injected into context")
		}
		if captured.ID != "user-1" {
			t.Errorf("user.ID = %q, want %q", captured.ID, "user-1")
		}
	})
}

func TestUserFromCtx_PanicsWithoutMiddleware(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when user is not in context")
		}
	}()
	middleware.UserFromCtx(context.Background())
}
