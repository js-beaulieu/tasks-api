package middleware

import (
	"context"
	"net/http"

	"github.com/js-beaulieu/tasks/internal/httpserver/render"
	"github.com/js-beaulieu/tasks/internal/model"
	"github.com/js-beaulieu/tasks/internal/repo"
)

type ctxKey struct{}

var userCtxKey = ctxKey{}

// AuthMiddleware reads X-User-ID (required), X-User-Name, and X-User-Email headers.
// Calls UserRepo.GetOrCreate and injects *model.User into the request context.
// Returns 401 JSON if X-User-ID is absent.
func AuthMiddleware(users repo.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-User-ID")
			if id == "" {
				render.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			name := r.Header.Get("X-User-Name")
			email := r.Header.Get("X-User-Email")

			u, err := users.GetOrCreate(r.Context(), id, name, email)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "internal error")
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, u)
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
