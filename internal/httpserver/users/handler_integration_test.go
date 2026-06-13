//go:build integration

package users_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
)

func TestGetMe_ExistingUser(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/users/me", Body: "", UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

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

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/users/me", Body: "", UserID: "gw-user-1", Headers: map[string]string{
		"X-User-Name":  "Gateway User",
		"X-User-Email": "gw@example.com",
	}})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

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

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/users/me", Body: `{"name":"Updated","email":"updated@example.com"}`, UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

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

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/users/me", Body: `{"name":"   "}`, UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

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

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/users/me", Body: `{"email":""}`, UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

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

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodPatch, Path: "/users/me", Body: `not-json`, UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["error"] != "invalid JSON" {
		t.Errorf("error = %q, want %q", body["error"], "invalid JSON")
	}
}

func TestGetUserByID_Existing(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/users/" + env.User.ID, Body: "", UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

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

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/users/nonexistent-id", Body: "", UserID: env.User.ID, Headers: nil})
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["error"] != "not found" {
		t.Errorf("error = %q, want %q", body["error"], "not found")
	}
}
