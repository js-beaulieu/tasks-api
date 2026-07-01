package role

import "testing"

func TestRankRequire(t *testing.T) {
	r := Rank{"read": 1, "modify": 2, "admin": 3}
	cases := []struct {
		min, actual string
		want        bool
	}{
		{"read", "read", true},
		{"read", "modify", true},
		{"read", "admin", true},
		{"modify", "read", false},
		{"modify", "modify", true},
		{"admin", "modify", false},
		{"modify", "unknown", false},
		// The current implementation treats unknown roles as rank 0, so an
		// unknown minimum is always satisfied and an unknown actual never is.
		{"unknown", "admin", true},
		{"unknown", "unknown", true},
	}
	for _, c := range cases {
		if got := r.Require(c.min, c.actual); got != c.want {
			t.Errorf("Require(%q,%q) = %v, want %v", c.min, c.actual, got, c.want)
		}
	}
}

func TestRankValid(t *testing.T) {
	r := Rank{"read": 1}
	if !r.Valid("read") {
		t.Errorf("Valid(\"read\") = false, want true")
	}
	if r.Valid("admin") {
		t.Errorf("Valid(\"admin\") = false")
	}
}
