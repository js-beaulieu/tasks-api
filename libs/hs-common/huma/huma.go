// Package huma provides a small Huma test helper used by Home Stack API services.
package huma

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// NewTestMux creates a standalone Huma API for unit tests.
// Generated docs/schema routes are disabled to keep leaf-router tests focused
// on the routes under test and avoid wildcard path conflicts.
func NewTestMux(title string) (*http.ServeMux, huma.API) {
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig(title, "1.0.0")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	return mux, humago.New(mux, cfg)
}
