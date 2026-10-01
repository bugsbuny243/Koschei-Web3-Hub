package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/handlers"
)

func TestDeploymentRevisionFromAcceptsOnlyFortyHex(t *testing.T) {
	good := strings.Repeat("a", 40)
	if got := deploymentRevisionFrom(func(string) string { return "  " + strings.ToUpper(good) + "  " }); got != good {
		t.Fatalf("revision=%q want=%q", got, good)
	}
	for _, bad := range []string{"", "abc", strings.Repeat("z", 40), strings.Repeat("a", 39), strings.Repeat("a", 41)} {
		if got := deploymentRevisionFrom(func(string) string { return bad }); got != "" {
			t.Fatalf("invalid revision %q accepted as %q", bad, got)
		}
	}
}

func TestVersionEndpointReportsValidatedRailwayRevision(t *testing.T) {
	revision := strings.Repeat("b", 40)
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", revision)

	mux := http.NewServeMux()
	registerCoreRoutes(mux, &handlers.Handler{}, func(next http.HandlerFunc) http.HandlerFunc { return next })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["revision"] != revision || payload["status"] != "ok" {
		t.Fatalf("unexpected version payload: %#v", payload)
	}
}

func TestVersionEndpointOmitsUntrustedRevision(t *testing.T) {
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "not-a-commit")

	mux := http.NewServeMux()
	registerCoreRoutes(mux, &handlers.Handler{}, func(next http.HandlerFunc) http.HandlerFunc { return next })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["revision"]; ok {
		t.Fatalf("invalid platform revision leaked: %#v", payload)
	}
}
