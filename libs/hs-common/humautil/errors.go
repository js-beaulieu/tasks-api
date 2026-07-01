// Package humautil provides Huma-specific utility helpers for Home Stack API services.
package humautil

import (
	"errors"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/teambition/rrule-go"

	"github.com/js-beaulieu/hs-api/libs/hs-common/repo"
	"github.com/js-beaulieu/hs-api/libs/hs-common/role"
)

// Rank must be populated by each service with its own role names.
// The tasks API assigns it from model.RoleRead/RoleModify/RoleAdmin in server wiring.
var Rank = role.Rank{}

// RepoError converts a repository sentinel error into a Huma status error.
func RepoError(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
		return huma.Error404NotFound("not found")
	}
	if errors.Is(err, repo.ErrNoAccess) {
		return huma.Error403Forbidden("forbidden")
	}
	return huma.Error500InternalServerError("internal error")
}

// RequireRole returns true if actual meets or exceeds min according to the shared rank map.
func RequireRole(min, actual string) bool { return Rank.Require(min, actual) }

// ValidRole reports whether the rank map defines the given role.
func ValidRole(r string) bool { return Rank.Valid(r) }

// ValidateRecurrence checks that a recurrence rule string is valid and that
// recurring tasks have a due_date. It is shared because recurrence rule parsing
// is domain-agnostic RFC-5545 logic.
func ValidateRecurrence(recurrence *string, dueDate *string) error {
	if recurrence == nil || strings.TrimSpace(*recurrence) == "" {
		return nil
	}
	if _, err := rrule.StrToRRule(*recurrence); err != nil {
		return huma.Error422UnprocessableEntity("invalid recurrence rule")
	}
	if dueDate == nil {
		return huma.Error422UnprocessableEntity("recurring tasks require a due_date")
	}
	return nil
}
