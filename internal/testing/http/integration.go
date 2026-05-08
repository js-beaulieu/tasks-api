package httptestutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/config"
	"github.com/js-beaulieu/tasks-api/internal/httpserver"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
)

type Env struct {
	Store   *postgres.Store
	Handler http.Handler
	User    *model.User
}

func NewEnv(t *testing.T) *Env {
	t.Helper()

	_, store := testdb.Open(t)
	user, err := store.Users.Create(t.Context(), "u-http-1", "HTTP User", "http-user@example.com")
	if err != nil {
		t.Fatalf("seed HTTP user: %v", err)
	}
	return &Env{
		Store:   store,
		Handler: httpserver.New(store, config.Config{}),
		User:    user,
	}
}

func Request(t *testing.T, handler http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func AssertStatus(t *testing.T, res *httptest.ResponseRecorder, want int) {
	t.Helper()

	if res.Code != want {
		t.Fatalf("status = %d, want %d, body: %s", res.Code, want, res.Body.String())
	}
}

func Decode(t *testing.T, res *httptest.ResponseRecorder, v any) {
	t.Helper()

	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, res.Body.String())
	}
}
