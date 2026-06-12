package render_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/render"
)

func TestError(t *testing.T) {
	t.Run("encodes error field with given status", func(t *testing.T) {
		w := httptest.NewRecorder()
		render.Error(w, http.StatusBadRequest, "something went wrong")

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
		var got map[string]string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got["error"] != "something went wrong" {
			t.Errorf(`body["error"] = %q, want "something went wrong"`, got["error"])
		}
	})
}
