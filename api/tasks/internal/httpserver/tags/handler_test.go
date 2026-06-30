package tags_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/tags"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	httptestutil "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/http"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/testing/mock"
)

var testUser = &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	userRepo := &mock.UserRepo{User: testUser}
	middleware.AuthMiddleware(userRepo)(handler).ServeHTTP(w, req)
	return w
}

func newRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-User-ID", testUser.ID)
	req.Header.Set("X-User-Name", testUser.Name)
	req.Header.Set("X-User-Email", testUser.Email)
	return req
}

func newHandler(tagRepo *mock.TagRepo) http.Handler {
	mux, api := httptestutil.NewHumaMux("tasks-api-tags-test")
	tags.RegisterRoutes(api, tagRepo, "")
	return mux
}

// ── GET /tags ─────────────────────────────────────────────────────────────

func TestListTags(t *testing.T) {
	t.Run("GET / returns distinct sorted tags for user", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListDistinctForUserFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{"bug", "feature", "urgent"}, nil
			},
		}
		handler := newHandler(tagRepo)
		w := serve(handler, newRequest(http.MethodGet, "/"))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got []string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("len = %d, want 3", len(got))
		}
	})

	t.Run("GET / with no tags returns empty array", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListDistinctForUserFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{}, nil
			},
		}
		handler := newHandler(tagRepo)
		w := serve(handler, newRequest(http.MethodGet, "/"))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var got []string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("expected empty array, got %v", got)
		}
	})
}
