package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	"github.com/js-beaulieu/tasks/internal/logger"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

type ctxKey struct{}

var userCtxKey = ctxKey{}

// AuthMiddleware reads the X-User-ID header and looks up the user by ID.
// Returns 401 if the header is absent or no matching user exists.
// Users are never created implicitly — use POST /login for first-time registration.
func AuthMiddleware(users repo.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-User-ID")
			if id == "" {
				render.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			u, err := users.GetByID(r.Context(), id)
			if err != nil {
				if errors.Is(err, repo.ErrNotFound) {
					render.Error(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				render.Error(w, http.StatusInternalServerError, "internal error")
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, u)
			log := logger.FromCtx(ctx).With("user_id", u.ID)
			ctx = logger.IntoCtx(ctx, log)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromCtx retrieves the authenticated *model.User from the context.
// Panics if not present — AuthMiddleware must run before any handler that calls this.
func UserFromCtx(ctx context.Context) *model.User {
	u, ok := ctx.Value(userCtxKey).(*model.User)
	if !ok || u == nil {
		panic("middleware.UserFromCtx: user not in context — AuthMiddleware must run first")
	}
	return u
}
