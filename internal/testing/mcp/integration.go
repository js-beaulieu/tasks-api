package mcptest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/js-beaulieu/tasks-api/internal/config"
	httpmdw "github.com/js-beaulieu/tasks-api/internal/httpserver/middleware"
	"github.com/js-beaulieu/tasks-api/internal/mcpserver"
	"github.com/js-beaulieu/tasks-api/internal/model"
	"github.com/js-beaulieu/tasks-api/internal/store/postgres"
	testdb "github.com/js-beaulieu/tasks-api/internal/testing/db"
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

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(httpmdw.AuthMiddleware(store.Users))
		r.Handle("/mcp", mcpserver.Handler(store, config.Config{}))
	})
	server := httptest.NewServer(r)

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

func DecodeStructured[T any](t *testing.T, result *mcp.CallToolResult) T {
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

func TextJSON(t *testing.T, result *mcp.CallToolResult, v any) {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("content is empty")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] = %T, want *mcp.TextContent", result.Content[0])
	}
	if err := json.Unmarshal([]byte(text.Text), v); err != nil {
		t.Fatalf("decode text JSON: %v; text: %s", err, text.Text)
	}
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
