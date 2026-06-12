package render

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func HumaConfig() huma.Config {
	cfg := huma.DefaultConfig("Tasks API", "1.0.0")
	cfg.CreateHooks = nil
	cfg.Transformers = nil
	return cfg
}
