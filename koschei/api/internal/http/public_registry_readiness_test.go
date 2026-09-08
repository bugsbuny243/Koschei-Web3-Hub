package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicCasesOwnsDatabaseOptionalDegradedMode(t *testing.T) {
	if !allowedWithoutDatabase("/api/public/cases") {
		t.Fatal("/api/public/cases must reach its portable backend selector when the application DB is absent")
	}
	if allowedWithoutDatabase("/api/public/soc/feed") {
		t.Fatal("/api/public/soc/feed must remain database-required until it has its own portable evidence backend")
	}
}

func TestAPIReadinessPassesPublicCasesWithoutDatabase(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/public/cases?limit=100", nil)
	recorder := httptest.NewRecorder()

	apiReadiness(nil, next).ServeHTTP(recorder, req)

	if !called {
		t.Fatal("portable public registry handler was blocked by generic database readiness")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", recorder.Code, http.StatusNoContent)
	}
}
