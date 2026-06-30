package httptestutil

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// NewHumaMux creates a standalone Huma API for unit tests.
// Generated docs/schema routes are disabled to keep leaf-router tests focused
// on the routes under test and avoid wildcard path conflicts.
func NewHumaMux(title string) (*http.ServeMux, huma.API) {
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig(title, "1.0.0")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	return mux, humago.New(mux, cfg)
}
