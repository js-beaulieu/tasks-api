package mcptest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	httpmdw "github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver/middleware"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/mcpserver"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/store/postgres"
	testdb "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/db"
	"github.com/js-beaulieu/hs-api/libs/hs-common/config"
)

type Env struct {
	Store   *postgres.Store
	User    *model.User
	Server  *httptest.Server
	Session *mcp.ClientSession
}

func NewEnv(t *testing.T) *Env {
	t.Helper()

	_, store := testdb.Open(t)
	user, err := store.Users.Create(t.Context(), "u-mcp-1", "MCP User", "mcp-user@example.com")
	if err != nil {
		t.Fatalf("seed MCP user: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", httpmdw.AuthMiddleware(store.Users)(mcpserver.Handler(store, config.Config{})))
	server := httptest.NewServer(mux)

	client := mcp.NewClient(&mcp.Implementation{Name: "tasks-api-test-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             server.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: userIDTransport{userID: user.ID, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		server.Close()
		t.Fatalf("connect MCP client: %v", err)
	}

	t.Cleanup(func() {
		_ = session.Close()
		server.Close()
	})

	return &Env{
		Store:   store,
		User:    user,
		Server:  server,
		Session: session,
	}
}

func CallTool(t *testing.T, env *Env, name string, args any) *mcp.CallToolResult {
	t.Helper()

	result, err := env.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call tool %q: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("tool %q returned MCP error: %v", name, result.GetError())
	}
	return result
}

func Decode[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()

	var out T
	if result.StructuredContent == nil {
		t.Fatal("structured content is nil")
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode structured content: %v; content: %s", err, data)
	}
	return out
}

type userIDTransport struct {
	userID string
	base   http.RoundTripper
}

func (t userIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("X-User-ID", t.userID)
	return t.base.RoundTrip(clone)
}
