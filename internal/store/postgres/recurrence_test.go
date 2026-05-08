package postgres

import (
	"testing"
	"time"
)

func TestRecurrence_NextOccurrence(t *testing.T) {
	tests := []struct {
		name       string
		due        string
		rrule      string
		want       string
		wantErr    bool
		validateFn func(t *testing.T, got string)
	}{
		{
			name:  "daily: Monday to Tuesday",
			due:   "2026-04-14",
			rrule: "FREQ=DAILY",
			want:  "2026-04-15",
		},
		{
			name:  "weekdays: Friday to Monday",
			due:   "2026-04-17",
			rrule: "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR",
			want:  "2026-04-20",
		},
		{
			name:  "weekdays: Monday to Tuesday",
			due:   "2026-04-14",
			rrule: "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR",
			want:  "2026-04-15",
		},
		{
			name:  "biweekly",
			due:   "2026-04-14",
			rrule: "FREQ=WEEKLY;INTERVAL=2",
			want:  "2026-04-28",
		},
		{
			name:  "monthly on Jan 31 — valid date, not hardcoded",
			due:   "2026-01-31",
			rrule: "FREQ=MONTHLY",
			validateFn: func(t *testing.T, got string) {
				t.Helper()
				if _, err := time.Parse("2006-01-02", got); err != nil {
					t.Errorf("got invalid date %q: %v", got, err)
				}
			},
		},
		{
			name:  "yearly",
			due:   "2026-04-14",
			rrule: "FREQ=YEARLY",
			want:  "2027-04-14",
		},
		{
			name:    "bogus rule returns error",
			due:     "2026-04-14",
			rrule:   "BOGUS=GARBAGE",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextOccurrence(tc.due, tc.rrule)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got result %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != "" && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if tc.validateFn != nil {
				tc.validateFn(t, got)
			}
		})
	}
}
