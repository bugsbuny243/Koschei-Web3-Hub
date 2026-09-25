package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/runtimehealth"
)

func TestSecurityCenterCapabilityRoute(t *testing.T) {
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/fabric/security-center/capabilities", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	for _, want := range []string{"koschei.security-center-capability.v1", "global-radar-event-plane", "preserve_existing"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("capability response missing %q", want)
		}
	}
}

func TestSecurityCenterSurfaceExplainsEvidenceBoundary(t *testing.T) {
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/fabric/security-center", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	for _, want := range []string{"KOSCHEI GLOBAL CRYPTO SECURITY CENTER", "Unknown stays unknown", "Preserved runtime assets"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("security-center surface missing %q", want)
		}
	}
}


func TestSecurityCenterRuntimeHealthRoute(t *testing.T) {
	registry := runtimehealth.New()
	registry.Register("worker.global-radar-head-ingest", "worker", "", true)
	registry.Success("worker.global-radar-head-ingest", 2)

	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux, registry)

	req := httptest.NewRequest(http.MethodGet, "/fabric/security-center/runtime-health", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	for _, want := range []string{"koschei.runtime-health.v1", "worker.global-radar-head-ingest", "state"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("runtime health response missing %q: %s", want, res.Body.String())
		}
	}
	if !strings.Contains(res.Body.String(), "live") {
		t.Fatalf("runtime health response did not report live state: %s", res.Body.String())
	}
}
