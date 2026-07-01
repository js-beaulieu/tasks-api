package repo

import "testing"

func TestSentinels(t *testing.T) {
	if ErrNotFound == nil {
		t.Error("ErrNotFound should not be nil")
	}
	if ErrNoAccess == nil {
		t.Error("ErrNoAccess should not be nil")
	}
	if ErrConflict == nil {
		t.Error("ErrConflict should not be nil")
	}
	if ErrNotFound.Error() != "not found" {
		t.Errorf("ErrNotFound message = %q, want not found", ErrNotFound.Error())
	}
}
