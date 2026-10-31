package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/handlers"
)

func TestOwnerSOCReadinessRequiresOwnerAndIsGetOnly(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-readiness-owner-fixture")
	t.Setenv("OWNER_WALLET", "")
	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{}, t.TempDir())

	anonymous := httptest.NewRecorder()
	mux.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/owner/soc/readiness", nil))
	if anonymous.Code != http.StatusNotFound {
		t.Fatalf("anonymous readiness returned %d, want 404", anonymous.Code)
	}

	auth := httptest.NewRequest(http.MethodGet, "/api/owner/soc/readiness", nil)
	auth.Header.Set("x-koschei-secret", "soc-readiness-owner-fixture")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, auth)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("authenticated request without a DB must return 503, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "soc-readiness-owner-fixture") {
		t.Fatal("owner credential leaked through readiness response")
	}

	post := httptest.NewRequest(http.MethodPost, "/api/owner/soc/readiness", nil)
	post.Header.Set("x-koschei-secret", "soc-readiness-owner-fixture")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("readiness endpoint must be GET only; got %d", rec.Code)
	}
}

func TestOwnerSOCVerdictCandidatesRequiresOwnerAndIsGetOnly(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-verdict-owner-test")
	t.Setenv("OWNER_WALLET", "")
	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{}, t.TempDir())
	path := "/api/owner/soc/verdict-candidates"

	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, path, nil))
	if unauthorized.Code != http.StatusNotFound {
		t.Fatalf("untrusted signed verdict list returned %d, want 404", unauthorized.Code)
	}
	get := httptest.NewRequest(http.MethodGet, path, nil)
	get.Header.Set("x-koschei-secret", "soc-verdict-owner-test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, get)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("authenticated signed list without database returned %d, want 503", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "soc-verdict-owner-test") {
		t.Fatal("SOC signed verdict endpoint leaked the operator credential")
	}
	post := httptest.NewRequest(http.MethodPost, path, nil)
	post.Header.Set("x-koschei-secret", "soc-verdict-owner-test")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("signed verdict candidate endpoint must be read-only: %d", rec.Code)
	}
}

func TestOwnerSOCTriageDisplaysReadinessSeparately(t *testing.T) {
	contents := ownerSOCTriagePageHTML
	for _, required := range []string{
		"/api/owner/soc/readiness",
		"/api/owner/soc/verdict-candidates",
		"id=\"runtime-readiness\"",
		"NOT end-to-end accepted",
		"UNAVAILABLE / NOT VERIFIED",
		"credentials:\"same-origin\"",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("owner UI missing readiness contract %q", required)
		}
	}
	if strings.Contains(contents, "innerHTML") {
		t.Fatal("owner readiness must not inject untrusted JSON as markup")
	}
}
