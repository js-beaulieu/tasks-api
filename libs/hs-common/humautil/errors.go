// Package humautil provides Huma-specific utility helpers for Home Stack API services.
package humautil

import (
	"errors"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/teambition/rrule-go"

	"github.com/js-beaulieu/hs-api/libs/hs-common/repo"
)

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
