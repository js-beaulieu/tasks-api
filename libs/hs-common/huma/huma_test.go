package huma

import (
	"net/http"
	"testing"
)

func TestNewTestMux(t *testing.T) {
	mux, api := NewTestMux("test-api")
	if mux == nil {
		t.Fatal("mux is nil")
	}
	if api == nil {
		t.Fatal("api is nil")
	}
	// NewTestMux returns *http.ServeMux as http.Handler.
	var _ http.Handler = mux
}
