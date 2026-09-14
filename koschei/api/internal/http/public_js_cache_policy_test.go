package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMutablePublicJavaScriptRequiresRevalidation(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := securityHeaders(next)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/js/customer-scan-entry.js?v=1", nil)
	handler.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("public JavaScript cache policy = %q, want no-cache", got)
	}
}

func TestPublicJavaScriptCachePolicyDoesNotLeakOntoAPIResponses(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := securityHeaders(next)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/scan", nil)
	handler.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("API response inherited public asset cache policy %q", got)
	}
}

func TestSensitiveStaticProbeStillOverridesWithNoStore(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := securityHeaders(next)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/js/.env.js", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("sensitive static probe status = %d, want 404", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("sensitive static probe cache policy = %q, want no-store", got)
	}
}
