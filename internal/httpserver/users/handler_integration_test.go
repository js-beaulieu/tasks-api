//go:build integration

package users_test

import (
	"net/http"
	"sort"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/model"
	httptestutil "github.com/js-beaulieu/tasks-api/internal/testing/http"
	"github.com/js-beaulieu/tasks-api/internal/testing/seed"
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
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
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
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
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
}

func TestListUsers_BatchLookup(t *testing.T) {
	env := httptestutil.NewEnv(t)

	u2 := seed.User(t, env.Store, seed.UserInput{ID: "u-batch-2", Name: "Bob", Email: "bob-batch@example.com"})
	u3 := seed.User(t, env.Store, seed.UserInput{ID: "u-batch-3", Name: "Carol", Email: "carol-batch@example.com"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?ids=" + env.User.ID + "&ids=" + u2.ID + "&ids=" + u3.ID,
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got []*model.User
	httptestutil.Decode(t, res, &got)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })
	want := []*model.User{env.User, u2, u3}
	sort.Slice(want, func(i, j int) bool { return want[i].ID < want[j].ID })
	for i := range got {
		if got[i].ID != want[i].ID {
			t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, want[i].ID)
		}
	}
}

func TestListUsers_NonExistentIDsOmitted(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?ids=" + env.User.ID + "&ids=nonexistent-id",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got []*model.User
	httptestutil.Decode(t, res, &got)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != env.User.ID {
		t.Errorf("ID = %q, want %q", got[0].ID, env.User.ID)
	}
}

func TestListUsers_EmptyIDsReturns422(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestListUsers_NoMatchesReturnsEmptyArray(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?ids=nonexistent-1&ids=nonexistent-2",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got []*model.User
	httptestutil.Decode(t, res, &got)
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestListUsers_SearchByName(t *testing.T) {
	env := httptestutil.NewEnv(t)

	seed.User(t, env.Store, seed.UserInput{ID: "u-search-1", Name: "Bob Builder", Email: "bob-builder@example.com"})
	seed.User(t, env.Store, seed.UserInput{ID: "u-search-2", Name: "Bobby Fischer", Email: "bobby@example.com"})
	seed.User(t, env.Store, seed.UserInput{ID: "u-search-3", Name: "Carol", Email: "carol@example.com"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?search=bob",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got []*model.User
	httptestutil.Decode(t, res, &got)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, u := range got {
		if u.ID == "u-search-3" {
			t.Errorf("Carol should not appear in search results for 'bob'")
		}
	}
}

func TestListUsers_SearchByEmail(t *testing.T) {
	env := httptestutil.NewEnv(t)

	seed.User(t, env.Store, seed.UserInput{ID: "u-search-4", Name: "Dave", Email: "dave-special@example.com"})
	seed.User(t, env.Store, seed.UserInput{ID: "u-search-5", Name: "Eve", Email: "eve@example.com"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?search=special",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got []*model.User
	httptestutil.Decode(t, res, &got)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "u-search-4" {
		t.Errorf("ID = %q, want %q", got[0].ID, "u-search-4")
	}
}

func TestListUsers_SearchRespectsLimit(t *testing.T) {
	env := httptestutil.NewEnv(t)

	seed.User(t, env.Store, seed.UserInput{ID: "u-search-6", Name: "Limit Alice", Email: "limit-alice@example.com"})
	seed.User(t, env.Store, seed.UserInput{ID: "u-search-7", Name: "Limit Bob", Email: "limit-bob@example.com"})
	seed.User(t, env.Store, seed.UserInput{ID: "u-search-8", Name: "Limit Carol", Email: "limit-carol@example.com"})

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?search=limit&limit=2",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var got []*model.User
	httptestutil.Decode(t, res, &got)
	if len(got) > 2 {
		t.Fatalf("len = %d, want <= 2", len(got))
	}
}

func TestListUsers_SearchAndIdsMutuallyExclusive(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{
		Method: http.MethodGet,
		Path:   "/users?search=bob&ids=u1",
		UserID: env.User.ID,
	})
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
	}
}
