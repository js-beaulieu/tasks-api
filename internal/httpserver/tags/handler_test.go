package tags_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/tags"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/testing/mock"
)

var testUser = &model.User{ID: "user-1", Name: "Alice", Email: "alice@example.com"}

func TestListTags(t *testing.T) {
	t.Run("GET /tags returns distinct sorted tags for user", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListDistinctForUserFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{"bug", "feature", "urgent"}, nil
			},
		}
		userRepo := &mock.UserRepo{User: testUser}
		handler := mock.NewTestRouter(userRepo, func(api huma.API) { tags.Register(api, tagRepo) })

		req := httptest.NewRequest(http.MethodGet, "/tags", nil)
		req.Header.Set("X-User-ID", testUser.ID)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

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

	t.Run("GET /tags with no tags returns empty array", func(t *testing.T) {
		tagRepo := &mock.TagRepo{
			ListDistinctForUserFn: func(_ context.Context, _ string) ([]string, error) {
				return []string{}, nil
			},
		}
		userRepo := &mock.UserRepo{User: testUser}
		handler := mock.NewTestRouter(userRepo, func(api huma.API) { tags.Register(api, tagRepo) })

		req := httptest.NewRequest(http.MethodGet, "/tags", nil)
		req.Header.Set("X-User-ID", testUser.ID)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

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
