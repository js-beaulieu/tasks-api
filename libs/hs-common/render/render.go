// Package render provides simple HTTP JSON response helpers for Home Stack API services.
package render

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a JSON error response with the given status and message.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// NotFound responds with 404 JSON error.
func NotFound(w http.ResponseWriter) {
	Error(w, http.StatusNotFound, "not found")
}

// Forbidden responds with 403 JSON error.
func Forbidden(w http.ResponseWriter) {
	Error(w, http.StatusForbidden, "forbidden")
}

// BadRequest responds with 400 JSON error.
func BadRequest(w http.ResponseWriter, msg string) {
	Error(w, http.StatusBadRequest, msg)
}

// UnprocessableEntity responds with 422 JSON error.
func UnprocessableEntity(w http.ResponseWriter, msg string) {
	Error(w, http.StatusUnprocessableEntity, msg)
}

// NoContent responds with 204 and a JSON content-type.
func NoContent(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
