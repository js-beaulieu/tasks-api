package postgres

import (
	"fmt"
	"time"

	"github.com/teambition/rrule-go"
)

const recurrenceDateLayout = "2006-01-02"

// nextOccurrence computes the next due date string given an ISO-8601 date and
// an RFC 5545 RRULE string. Returns an error if the date or rule cannot be parsed.
func nextOccurrence(due, rruleStr string) (string, error) {
	t, err := time.Parse(recurrenceDateLayout, due)
	if err != nil {
		return "", fmt.Errorf("parse due date: %w", err)
	}
	rule, err := rrule.StrToRRule(rruleStr)
	if err != nil {
		return "", fmt.Errorf("parse rrule: %w", err)
	}
	rule.DTStart(t)
	next := rule.After(t, false)
	if next.IsZero() {
		return "", fmt.Errorf("rrule produced no next occurrence")
	}
	return next.Format(recurrenceDateLayout), nil
}
