package httptestutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/js-beaulieu/hs-api/api/tasks/internal/httpserver"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/model"
	"github.com/js-beaulieu/hs-api/api/tasks/internal/store/postgres"
	testdb "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/db"
	"github.com/js-beaulieu/hs-api/libs/hs-common/config"
)

type Env struct {
	Store   *postgres.Store
	BaseURL string
	Client  *http.Client
	User    *model.User
}

type RequestOptions struct {
	Method  string
	Path    string
	Body    any
	UserID  string
	Headers map[string]string
}

func NewEnv(t *testing.T) *Env {
	t.Helper()

	_, store := testdb.Open(t)
	user, err := store.Users.Create(t.Context(), "u-http-1", "HTTP User", "http-user@example.com")
	if err != nil {
		t.Fatalf("seed HTTP user: %v", err)
	}
	server := httptest.NewServer(httpserver.New(store, config.Config{}))
	t.Cleanup(server.Close)
	return &Env{
		Store:   store,
		BaseURL: server.URL,
		Client:  server.Client(),
		User:    user,
	}
}

func Request(t *testing.T, env *Env, opts RequestOptions) *http.Response {
	t.Helper()

	var reader io.Reader
	headers := map[string]string{}
	if opts.UserID != "" {
		headers["X-User-ID"] = opts.UserID
	}
	for k, v := range opts.Headers {
		headers[k] = v
	}
	switch v := opts.Body.(type) {
	case nil:
	case io.Reader:
		reader = v
	case string:
		reader = bytes.NewBufferString(v)
		if v != "" && headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	case []byte:
		reader = bytes.NewReader(v)
		if len(v) > 0 && headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	default:
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(v); err != nil {
			t.Fatalf("encode request JSON: %v", err)
		}
		reader = &buf
		if headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	}

	req, err := http.NewRequestWithContext(t.Context(), opts.Method, env.BaseURL+opts.Path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	t.Cleanup(func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	})
	return res
}

func Decode(t *testing.T, res *http.Response, v any) {
	t.Helper()

	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
