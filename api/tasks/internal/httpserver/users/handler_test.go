package users_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/users"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	httptestutil "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/http"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/mock"
)

// authed wraps a handler with AuthMiddleware backed by the given mock.
func authed(m *mock.UserRepo, h http.Handler) http.Handler {
	return middleware.AuthMiddleware(m)(h)
}

func newHandler(m *mock.UserRepo) http.Handler {
	mux, api := httptestutil.NewHumaMux("tasks-api-users-test")
	users.RegisterRoutes(api, m, "")
	return mux
}

func TestGetMe(t *testing.T) {
	t.Run("valid X-User-ID returns 200 and user JSON", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, newHandler(m))

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
		handler := authed(m, newHandler(m))

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
		handler := authed(m, newHandler(m))

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

}

func TestSearchUsers(t *testing.T) {
	t.Run("search returns matching users", func(t *testing.T) {
		users := []*model.User{
			{ID: "user-2", Name: "Bob", Email: "bob@example.com"},
			{ID: "user-3", Name: "Carol", Email: "carol@example.com"},
		}
		m := &mock.UserRepo{
			User:  &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"},
			Users: users,
			SearchFn: func(_ context.Context, query string, limit int) ([]*model.User, error) {
				if query != "bob" {
					t.Errorf("query = %q, want %q", query, "bob")
				}
				return users, nil
			},
		}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodGet, "/?search=bob", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got []*model.User
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("search with ids returns 422", func(t *testing.T) {
		m := &mock.UserRepo{User: &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodGet, "/?search=bob&ids=user-2", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", w.Code)
		}
	})

	t.Run("search with no results returns empty array", func(t *testing.T) {
		m := &mock.UserRepo{
			User:  &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"},
			Users: nil,
			SearchFn: func(_ context.Context, _ string, _ int) ([]*model.User, error) {
				return nil, nil
			},
		}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodGet, "/?search=nobody", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got []*model.User
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("search repo error returns 500", func(t *testing.T) {
		m := &mock.UserRepo{
			User: &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"},
			SearchFn: func(_ context.Context, _ string, _ int) ([]*model.User, error) {
				return nil, errors.New("db error")
			},
		}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodGet, "/?search=bob", nil)
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
	})
}

func TestUpdateMe(t *testing.T) {
	t.Run("valid patch returns updated user", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, newHandler(m))

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

	t.Run("blank name returns 422", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"name":"   "}`))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", w.Code)
		}
	})

	t.Run("blank email returns 422", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"email":""}`))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", w.Code)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		u := &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}
		m := &mock.UserRepo{User: u}
		handler := authed(m, newHandler(m))

		req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`not-json`))
		req.Header.Set("X-User-ID", "user-1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}
