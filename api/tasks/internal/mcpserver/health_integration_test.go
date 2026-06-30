//go:build integration

package mcpserver_test

import (
	"testing"

	mcptest "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPHealthIntegration(t *testing.T) {
	env := mcptest.NewEnv(t)

	result := mcptest.CallTool(t, env, "health", nil)
	body := mcptest.Decode[map[string]string](t, result)
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}

func TestMCPToolsListSmoke(t *testing.T) {
	env := mcptest.NewEnv(t)

	listResult, err := env.Session.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	names := map[string]bool{}
	for _, tool := range listResult.Tools {
		names[tool.Name] = true
	}

	for _, want := range []string{"health", "list_projects", "list_tasks", "list_tags"} {
		if !names[want] {
			t.Errorf("missing expected tool %q; registered: %v", want, toolNames(listResult))
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

func toolNames(r *mcp.ListToolsResult) []string {
	out := make([]string, len(r.Tools))
	for i, t := range r.Tools {
		out[i] = t.Name
	}
	return out
}
