package middleware

import (
	"net/http"

	"github.com/js-beaulieu/hs-api/libs/hs-common/config"
	"github.com/js-beaulieu/hs-api/libs/hs-common/logmw"
)

// Logging returns the shared request-logging middleware configured for the app.
func Logging(cfg config.Config) func(http.Handler) http.Handler {
	return logmw.Middleware(cfg)
}
