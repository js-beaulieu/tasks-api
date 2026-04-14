package render_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/tasks/internal/httpserver/render"
)

func TestJSON(t *testing.T) {
	t.Run("sets Content-Type, status, and body", func(t *testing.T) {
		w := httptest.NewRecorder()
		render.JSON(w, http.StatusCreated, map[string]string{"key": "value"})

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var got map[string]string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got["key"] != "value" {
			t.Errorf("body[key] = %q, want %q", got["key"], "value")
		}
	})
}

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

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	render.NotFound(w)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	var got map[string]string
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if got["error"] != "not found" {
		t.Errorf(`body["error"] = %q, want "not found"`, got["error"])
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	render.Forbidden(w)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	var got map[string]string
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if got["error"] != "forbidden" {
		t.Errorf(`body["error"] = %q, want "forbidden"`, got["error"])
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	render.BadRequest(w, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var got map[string]string
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if got["error"] != "invalid input" {
		t.Errorf(`body["error"] = %q, want "invalid input"`, got["error"])
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	render.NoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", w.Body.String())
	}
}
