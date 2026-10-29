package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/handlers"
)

func TestOwnerSOCTriagePageRejectsAnonymousAccess(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-test-owner-secret")
	t.Setenv("OWNER_WALLET", "")

	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{}, t.TempDir())
	serve := securityHeaders(mux)

	rec := httptest.NewRecorder()
	serve.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/owner/soc-triage", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("anonymous SOC view returned %d; want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Owner SOC triage") {
		t.Fatal("private SOC view leaked to anonymous caller")
	}

	auth := httptest.NewRequest(http.MethodGet, "/owner/soc-triage", nil)
	auth.Header.Set("x-koschei-secret", "soc-test-owner-secret")
	rec = httptest.NewRecorder()
	serve.ServeHTTP(rec, auth)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated SOC view returned %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Owner SOC triage") ||
		!strings.Contains(rec.Body.String(), "triage=1&limit=100") {
		t.Fatal("owner view is missing read-only SOC queue contract")
	}
	if !strings.Contains(rec.Body.String(), "No verified analyst") ||
		!strings.Contains(rec.Body.String(), "SLA: NOT CONFIGURED") ||
		!strings.Contains(rec.Body.String(), "No owner review record") ||
		!strings.Contains(rec.Body.String(), "Manual owner reviews are not analyst responses") {
		t.Fatal("owner view must preserve non-verified assignment, review and SLA semantics")
	}
	if !strings.Contains(rec.Body.String(), "Open handoff JSON") ||
		!strings.Contains(rec.Body.String(), "?handoff=1") {
		t.Fatal("owner view must link to the private, read-only handoff endpoint")
	}
	if strings.Contains(rec.Body.String(), "innerHTML") ||
		strings.Contains(rec.Body.String(), "eval(") {
		t.Fatal("owner SOC UI must not execute untrusted case content")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner SOC view must not be cached")
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "nonce-") ||
		!strings.Contains(rec.Body.String(), "<script nonce=") {
		t.Fatal("owner SOC scripts must receive response-specific CSP nonce")
	}

	post := httptest.NewRequest(http.MethodPost, "/owner/soc-triage", nil)
	post.Header.Set("x-koschei-secret", "soc-test-owner-secret")
	rec = httptest.NewRecorder()
	serve.ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("SOC page is GET-only; POST returned %d", rec.Code)
	}
}
