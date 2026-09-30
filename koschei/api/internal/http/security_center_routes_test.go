package http

import (
	"errors"
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

func TestSecurityCenterShowsOperationalEvidenceWithoutProviderErrors(t *testing.T) {
	r := runtimehealth.New()
	r.Register("worker.global-radar-head-ingest", "worker", "", false)
	r.Register("storage.database", "storage", "", true)
	r.Success("storage.database", 0)
	r.Register("<script>component</script>", "worker", "", true)
	r.Failure("<script>component</script>", errors.New("private-provider-detail"))
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux, r)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center", nil))
	body := res.Body.String()
	for _, want := range []string{"Radar operational health", "disabled: 1", "inactive", "not_monitored", "No successful check", "0 observations", "&lt;script&gt;component&lt;/script&gt;"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in operational panel", want)
		}
	}
	if strings.Contains(body, "private-provider-detail") || strings.Contains(body, "<script>component</script>") {
		t.Fatal("operational panel exposed raw provider errors or unescaped IDs")
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("operational snapshot must not be cached")
	}
}

func TestSecurityCenterMissingRegistryDoesNotClaimHealth(t *testing.T) {
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center", nil))
	if !strings.Contains(res.Body.String(), "Runtime health unavailable. No registered component evidence.") {
		t.Fatal("missing health evidence must remain explicit")
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

func TestSecurityCenterRendersZeroPendingBlocks(t *testing.T) {
	r := runtimehealth.New()
	r.Register("head", "head_ingest", "ethereum-mainnet", true)
	height := uint64(100)
	r.RecordIngestProgress("head", &height, &height, false, false)
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux, r)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center", nil))
	for _, want := range []string{"Observed head: 100", "Durable cursor: 100", "Pending blocks: 0", "zero does not prove historical completeness"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("missing %q", want)
		}
	}
}


func TestSecurityCenterSurfacesCoverageAttentionWithoutClaimingMissingHistory(t *testing.T) {
	r := runtimehealth.New()
	r.Register("head", "head_ingest", "ethereum-mainnet", true)
	head, cursor := uint64(99), uint64(100)
	r.RecordIngestProgress("head", &head, &cursor, true, false)

	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux, r)

	htmlRes := httptest.NewRecorder()
	mux.ServeHTTP(htmlRes, httptest.NewRequest(http.MethodGet, "/fabric/security-center", nil))
	for _, want := range []string{"blind_spot", "provider_head_is_behind_durable_cursor", "attention required", "not proof that historical chain data is missing"} {
		if !strings.Contains(htmlRes.Body.String(), want) {
			t.Fatalf("security center missing coverage signal %q: %s", want, htmlRes.Body.String())
		}
	}

	jsonRes := httptest.NewRecorder()
	mux.ServeHTTP(jsonRes, httptest.NewRequest(http.MethodGet, "/fabric/security-center/runtime-health", nil))
	for _, want := range []string{`"coverage_status":"blind_spot"`, `"coverage_attention_required":true`, `"coverage_counts":{"blind_spot":1}`} {
		if !strings.Contains(jsonRes.Body.String(), want) {
			t.Fatalf("runtime health missing %q: %s", want, jsonRes.Body.String())
		}
	}
}
