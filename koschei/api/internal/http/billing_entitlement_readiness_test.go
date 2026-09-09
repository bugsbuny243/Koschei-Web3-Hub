package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/handlers"
)

func TestPolarWebhookBypassesApplicationDBButFailsClosedWithoutEntitlementStore(t *testing.T) {
	mux := http.NewServeMux()
	registerBillingRoutes(mux, &handlers.Handler{})

	request := httptest.NewRequest(http.MethodPost, "/api/polar/webhook", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	apiReadiness(nil, mux).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(response.Body.String(), "entitlement_store_unavailable") {
		t.Fatalf("body = %q, want entitlement-store failure", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatalf("billing route was incorrectly blocked by application DB readiness: %q", response.Body.String())
	}
}

func TestBillingPathsAreApplicationDBOptionalOnly(t *testing.T) {
	for path := range entitlementOnlyAPIPaths {
		if !allowedWithoutDatabase(path) {
			t.Fatalf("%s must bypass application DB readiness and rely on its entitlement gate", path)
		}
	}
}
