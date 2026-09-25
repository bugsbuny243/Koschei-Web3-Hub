package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
