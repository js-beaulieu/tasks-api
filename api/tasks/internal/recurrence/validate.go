// Package recurrence validates RFC-5545 recurrence rules for tasks.
package recurrence

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/teambition/rrule-go"
)

// Validate checks that a recurrence rule string is valid and that recurring
// tasks have a due_date. It is shared between project task creation and task
// mutation handlers because the recurrence rule grammar is domain-agnostic.
func Validate(recurrence *string, dueDate *string) error {
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
