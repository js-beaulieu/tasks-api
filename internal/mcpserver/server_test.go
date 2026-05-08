package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks-api/internal/config"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	"github.com/js-beaulieu/tasks-api/internal/testing/mock"
)

func TestHealthHandler(t *testing.T) {
	_, result, err := healthHandler(context.Background(), &mcp.CallToolRequest{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != "ok" {
		t.Fatalf("expected status=ok, got %q", result.Status)
	}
}

func TestNewServerHasHealthTool(t *testing.T) {
	s := New(nil, config.Config{})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServerWithStore(t *testing.T) {
	store := &postgres.Store{Users: &mock.UserRepo{}}
	s := New(store, config.Config{})
	if s == nil {
		t.Fatal("expected non-nil server with store")
	}
}

func TestHandler(t *testing.T) {
	store := &postgres.Store{Users: &mock.UserRepo{}}
	h := Handler(store, config.Config{})
	if h == nil {
		t.Fatal("expected non-nil http.Handler")
	}
}
