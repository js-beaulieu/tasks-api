//go:build integration

package httpserver_test

import (
	"net/http"
	"testing"

	"github.com/js-beaulieu/tasks-api/internal/httpserver/httptestutil"
)

func TestHealthIntegration(t *testing.T) {
	env := httptestutil.NewEnv(t)

	res := httptestutil.Request(t, env.Handler, http.MethodGet, "/health", "", "")
	httptestutil.AssertStatus(t, res, http.StatusOK)

	var body map[string]string
	httptestutil.Decode(t, res, &body)
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}
