package humautil

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/teambition/rrule-go"
)

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
