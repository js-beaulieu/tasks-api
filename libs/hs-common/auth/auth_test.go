package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/hs-api/libs/hs-common/repo"
)

type stubLoader struct {
	byID  map[string]*User
	calls []string
}

func (s *stubLoader) GetByID(ctx context.Context, id string) (*User, error) {
	s.calls = append(s.calls, "get:"+id)
	if u, ok := s.byID[id]; ok {
		return u, nil
	}
	return nil, repo.ErrNotFound
}

func (s *stubLoader) Create(ctx context.Context, id, name, email string) (*User, error) {
	s.calls = append(s.calls, "create:"+id)
	u := &User{ID: id, Name: name, Email: email}
	s.byID[id] = u
	return u, nil
}

func TestMiddlewareMissingUserID(t *testing.T) {
	handler := Middleware(
		&stubLoader{byID: map[string]*User{}},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without X-User-ID")
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareExistingUser(t *testing.T) {
	loader := &stubLoader{byID: map[string]*User{"u1": {ID: "u1", Name: "A", Email: "a@b"}}}
	var got *User
	handler := Middleware(loader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = UserFromCtx(r.Context())
	}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "u1")
	handler.ServeHTTP(w, req)
	if got == nil || got.ID != "u1" {
		t.Errorf("user = %+v, want u1", got)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestMiddlewareAutoProvision(t *testing.T) {
	loader := &stubLoader{byID: map[string]*User{}}
	var got *User
	handler := Middleware(loader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = UserFromCtx(r.Context())
	}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "u2")
	req.Header.Set("X-User-Name", "B")
	req.Header.Set("X-User-Email", "b@c")
	handler.ServeHTTP(w, req)
	if got == nil || got.ID != "u2" || got.Name != "B" || got.Email != "b@c" {
		t.Errorf("user = %+v, want u2", got)
	}
}

func TestWithUser(t *testing.T) {
	ctx := WithUser(context.Background(), &User{ID: "u3"})
	if u := UserFromCtx(ctx); u == nil || u.ID != "u3" {
		t.Errorf("user = %+v, want u3", u)
	}
}

func TestUserFromCtxMissingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic")
		}
	}()
	_ = UserFromCtx(context.Background())
}
