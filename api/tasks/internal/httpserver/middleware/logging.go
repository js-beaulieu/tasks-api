package middleware

import (
	"net/http"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/config"
	"github.com/js-beaulieu/hs-api/libs/hs-common/logmiddleware"
)

// Logging returns the shared request-logging middleware configured for the app.
func Logging(cfg config.Config) func(http.Handler) http.Handler {
	return logmiddleware.Middleware(logmiddleware.Options{Detailed: cfg.LogDetailed})
}
