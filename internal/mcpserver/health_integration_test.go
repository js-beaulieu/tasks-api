//go:build integration

package mcpserver_test

import (
	"testing"

	mcptest "github.com/js-beaulieu/tasks-api/internal/testing/mcp"
)

func TestMCPHealthIntegration(t *testing.T) {
	env := mcptest.NewEnv(t)

	result := mcptest.CallTool(t, env, "health", nil)
	body := mcptest.Decode[map[string]string](t, result)
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}
