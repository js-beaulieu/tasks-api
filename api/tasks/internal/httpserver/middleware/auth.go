package middleware

import (
	"context"
	"net/http"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/repo"
	"github.com/js-beaulieu/hs-api/libs/hs-common/auth"
)

// userCtxKey is the app-local context key used to store *model.User. The shared
// auth middleware stores a generic *auth.User under a different key, so this adapter
// re-stores the task-domain model after the shared middleware runs.
//
//nolint:gochecknoglobals
var userCtxKey = &struct{}{}

// userLoader adapts repo.UserRepo to the shared auth.UserLoader interface.
type userLoader struct{ users repo.UserRepo }

func (l userLoader) GetByID(ctx context.Context, id string) (*auth.User, error) {
	u, err := l.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &auth.User{ID: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (l userLoader) Create(ctx context.Context, id, name, email string) (*auth.User, error) {
	u, err := l.users.Create(ctx, id, name, email)
	if err != nil {
		return nil, err
	}
	return &auth.User{ID: u.ID, Name: u.Name, Email: u.Email}, nil
}

// AuthMiddleware reads the X-User-ID header and looks up the user by ID.
// If the user is not found, it auto-provisions them from X-User-Name and X-User-Email headers
// (set by the gateway after JWT validation). Returns 401 only if X-User-ID is absent.
//
// It wraps the shared auth.Middleware, then re-stores the user as the app-local
// *model.User so existing handlers and tests can keep using UserFromCtx.
func AuthMiddleware(users repo.UserRepo) func(http.Handler) http.Handler {
	shared := auth.Middleware(userLoader{users: users})
	return func(next http.Handler) http.Handler {
		return shared(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if au := auth.UserFromCtx(r.Context()); au != nil {
				r = r.WithContext(WithUser(r.Context(), &model.User{
					ID:    au.ID,
					Name:  au.Name,
					Email: au.Email,
				}))
			}
			next.ServeHTTP(w, r)
		}))
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

// WithUser returns ctx with u stored under the auth middleware's context key.
// For use in tests only.
func WithUser(ctx context.Context, u *model.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}
