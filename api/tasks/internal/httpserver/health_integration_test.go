//go:build integration

package httpserver_test

import (
	"net/http"
	"testing"

	httptestutil "github.com/js-beaulieu/hs-api/api/tasks/internal/testing/http"
)

func TestHealthIntegration(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env, httptestutil.RequestOptions{Method: http.MethodGet, Path: "/health", Body: nil, UserID: ""})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}
