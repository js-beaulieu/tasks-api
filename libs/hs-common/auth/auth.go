// Package auth provides a reusable request-scoped authentication middleware.
//
// The middleware trusts gateway-injected identity headers (X-User-ID, etc.)
// and does not perform browser OAuth, JWT verification, or token refresh.
package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/js-beaulieu/hs-api/libs/hs-common/logger"
	"github.com/js-beaulieu/hs-api/libs/hs-common/render"
	"github.com/js-beaulieu/hs-api/libs/hs-common/repo"
)

type ctxKey struct{}

var userCtxKey = ctxKey{}

// ContextKey returns the context key used by this package to store *User.
// It is exposed only to support binary-local adapter packages that also need
// to read or write the same key.
func ContextKey() any { return userCtxKey }

// User represents a minimal authenticated user entity.
// Services can use this directly or embed it in their own domain models.
type User struct {
	ID    string
	Name  string
	Email string
}

// UserLoader is the minimal dependency for the auth middleware.
// Callers typically adapt their concrete repo.UserRepo to this interface.
type UserLoader interface {
	GetByID(ctx context.Context, id string) (*User, error)
	Create(ctx context.Context, id, name, email string) (*User, error)
}

// Middleware reads X-User-ID and auto-provisions the user from X-User-Name
// and X-User-Email when absent. It returns 401 if X-User-ID is missing.
func Middleware(loader UserLoader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-User-ID")
			if id == "" {
				render.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			u, err := loader.GetByID(r.Context(), id)
			if err != nil {
				if !errors.Is(err, repo.ErrNotFound) {
					render.Error(w, http.StatusInternalServerError, "internal error")
					return
				}
				u, err = loader.Create(r.Context(), id,
					r.Header.Get("X-User-Name"),
					r.Header.Get("X-User-Email"),
				)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "internal error")
					return
				}
			}

			ctx := context.WithValue(r.Context(), userCtxKey, u)
			log := logger.FromCtx(ctx).With("user_id", u.ID)
			ctx = logger.IntoCtx(ctx, log)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareFunc returns the configured middleware as a function value suitable
// for frameworks that expect func(http.Handler) http.Handler. It is identical
// to Middleware but can be used as a value rather than an unconfigured factory.
func MiddlewareFunc(loader UserLoader) func(http.Handler) http.Handler {
	return Middleware(loader)
}

// UserFromCtx retrieves the authenticated *User from the context.
// Panics if not present — the middleware must run before any handler that calls this.
func UserFromCtx(ctx context.Context) *User {
	u, ok := ctx.Value(userCtxKey).(*User)
	if !ok || u == nil {
		panic("auth.UserFromCtx: user not in context — auth.Middleware must run first")
	}
	return u
}

// WithUser returns ctx with u stored under the auth middleware's context key.
// For use in tests only.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}
