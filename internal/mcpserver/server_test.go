package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHealthHandler(t *testing.T) {
	result, _, err := healthHandler(context.Background(), &mcp.CallToolRequest{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", result.Content[0])
	}

	var body map[string]string
	if err := json.Unmarshal([]byte(text.Text), &body); err != nil {
		t.Fatalf("failed to parse response text as JSON: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}

func TestNewServerHasHealthTool(t *testing.T) {
	s := New(nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
