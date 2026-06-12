//go:build integration

package users_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
)

func rawRequest(t *testing.T, handler http.Handler, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func requestWithHeaders(t *testing.T, handler http.Handler, method, path string, body string, userID string, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{}
	if userID != "" {
		headers["X-User-ID"] = userID
	}
	for k, v := range extra {
		headers[k] = v
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
		headers["Content-Type"] = "application/json"
	}
	return rawRequest(t, handler, method, path, reader, headers)
}

func TestGetMe_ExistingUser(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodGet, "/users/me", "", env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var got model.User
	httptestutil.Decode(t, res, &got)
	if got.ID != env.User.ID {
		t.Errorf("ID = %q, want %q", got.ID, env.User.ID)
	}
	if got.Name != env.User.Name {
		t.Errorf("Name = %q, want %q", got.Name, env.User.Name)
	}
	if got.Email != env.User.Email {
		t.Errorf("Email = %q, want %q", got.Email, env.User.Email)
	}
}

func TestGetMe_AutoProvisions(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodGet, "/users/me", "", "gw-user-1", map[string]string{
		"X-User-Name":  "Gateway User",
		"X-User-Email": "gw@example.com",
	})
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var got model.User
	httptestutil.Decode(t, res, &got)
	if got.ID != "gw-user-1" {
		t.Errorf("ID = %q, want %q", got.ID, "gw-user-1")
	}
	if got.Name != "Gateway User" {
		t.Errorf("Name = %q, want %q", got.Name, "Gateway User")
	}
	if got.Email != "gw@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "gw@example.com")
	}

	dbUser, err := env.Store.Users.GetByID(t.Context(), "gw-user-1")
	if err != nil {
		t.Fatalf("GetByID from DB: %v", err)
	}
	if dbUser.Name != "Gateway User" {
		t.Errorf("DB Name = %q, want %q", dbUser.Name, "Gateway User")
	}
	if dbUser.Email != "gw@example.com" {
		t.Errorf("DB Email = %q, want %q", dbUser.Email, "gw@example.com")
	}
}

func TestPatchMe_UpdatesNameAndEmail(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodPatch, "/users/me",
		`{"name":"Updated","email":"updated@example.com"}`, env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var got model.User
	httptestutil.Decode(t, res, &got)
	if got.Name != "Updated" {
		t.Errorf("response Name = %q, want %q", got.Name, "Updated")
	}
	if got.Email != "updated@example.com" {
		t.Errorf("response Email = %q, want %q", got.Email, "updated@example.com")
	}

	dbUser, err := env.Store.Users.GetByID(t.Context(), env.User.ID)
	if err != nil {
		t.Fatalf("GetByID from DB: %v", err)
	}
	if dbUser.Name != "Updated" {
		t.Errorf("DB Name = %q, want %q", dbUser.Name, "Updated")
	}
	if dbUser.Email != "updated@example.com" {
		t.Errorf("DB Email = %q, want %q", dbUser.Email, "updated@example.com")
	}
}

func TestPatchMe_BlankName(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodPatch, "/users/me",
		`{"name":"   "}`, env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusBadRequest)

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["error"] != "name cannot be blank" {
		t.Errorf("error = %q, want %q", body["error"], "name cannot be blank")
	}

	dbUser, err := env.Store.Users.GetByID(t.Context(), env.User.ID)
	if err != nil {
		t.Fatalf("GetByID from DB: %v", err)
	}
	if dbUser.Name != env.User.Name {
		t.Errorf("DB Name changed: got %q, want %q", dbUser.Name, env.User.Name)
	}
}

func TestPatchMe_BlankEmail(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodPatch, "/users/me",
		`{"email":""}`, env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusBadRequest)

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["error"] != "email cannot be blank" {
		t.Errorf("error = %q, want %q", body["error"], "email cannot be blank")
	}

	dbUser, err := env.Store.Users.GetByID(t.Context(), env.User.ID)
	if err != nil {
		t.Fatalf("GetByID from DB: %v", err)
	}
	if dbUser.Email != env.User.Email {
		t.Errorf("DB Email changed: got %q, want %q", dbUser.Email, env.User.Email)
	}
}

func TestPatchMe_InvalidJSON(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodPatch, "/users/me",
		`not-json`, env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusBadRequest)

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["error"] != "invalid JSON" {
		t.Errorf("error = %q, want %q", body["error"], "invalid JSON")
	}
}

func TestGetUserByID_Existing(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodGet, "/users/"+env.User.ID, "", env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var got model.User
	httptestutil.Decode(t, res, &got)
	if got.ID != env.User.ID {
		t.Errorf("ID = %q, want %q", got.ID, env.User.ID)
	}
	if got.Name != env.User.Name {
		t.Errorf("Name = %q, want %q", got.Name, env.User.Name)
	}
}

func TestGetUserByID_Missing(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := requestWithHeaders(t, env.Handler, http.MethodGet, "/users/nonexistent-id", "", env.User.ID, nil)
	httptestutil.AssertStatus(t, res, http.StatusNotFound)

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["error"] != "not found" {
		t.Errorf("error = %q, want %q", body["error"], "not found")
	}
}
