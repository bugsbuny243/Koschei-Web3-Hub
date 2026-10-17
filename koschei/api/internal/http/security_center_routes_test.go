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

// TestSecurityCenterAcceptanceRoute guarantees that the customer-visible catalog
// exposes all requested controls without claiming that missing proof is safe.
func TestSecurityCenterAcceptanceRoute(t *testing.T) {
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center/acceptance", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	for _, want := range []string{
		"koschei.security-center-acceptance.v1",
		`"area_count":34`, `"control_count":170`, `"common_gate_count":8`,
		`"verified_count":0`, `"id":"WSC-15.1"`, `"id":"WSC-34.5"`,
		`"id":"G08"`, "evidence_not_verified", "not_assessed",
	} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("acceptance response missing %q", want)
		}
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("acceptance register must not be cached")
	}
	post := httptest.NewRecorder()
	mux.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/fabric/security-center/acceptance", nil))
	if post.Code == http.StatusOK {
		t.Fatal("public acceptance endpoint must remain read-only")
	}
}

func TestSecurityCenterIntegrationReadinessRouteIsReadOnly(t *testing.T) {
	registry := runtimehealth.New()
	registry.Register("worker.global-radar-head-ingest", "head_ingest", "ethereum-mainnet", true)
	registry.Success("worker.global-radar-head-ingest", 0)
	mux := http.NewServeMux()
	registerSecurityCenterRoutes(mux, registry)

	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center/integration-readiness", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("readiness status=%d body=%s", res.Code, res.Body.String())
	}
	for _, token := range []string{
		"koschei.security-center-integration-readiness.v1",
		"continuous-multi-network-block-ingest",
		"observed_operational",
		"\"acceptance_verified\":false",
	} {
		if !strings.Contains(res.Body.String(), token) {
			t.Fatalf("readiness missing %q: %s", token, res.Body.String())
		}
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("integration readiness may not be cached")
	}
	post := httptest.NewRecorder()
	mux.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/fabric/security-center/integration-readiness", nil))
	if post.Code == http.StatusOK {
		t.Fatal("readiness route must not accept writes")
	}
}

func TestMountFabricReadinessPreservesOldRouteGraph(t *testing.T) {
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("original-base-route"))
	})
	mounted := MountFabric(base)
	newRoute := httptest.NewRecorder()
	mounted.ServeHTTP(newRoute, httptest.NewRequest(http.MethodGet, "/fabric/security-center/integration-readiness", nil))
	if newRoute.Code != http.StatusOK ||
		!strings.Contains(newRoute.Body.String(), "\"runtime_registry_available\":false") {
		t.Fatalf("mounted readiness did not fail closed: %d %s", newRoute.Code, newRoute.Body.String())
	}
	oldRoute := httptest.NewRecorder()
	mounted.ServeHTTP(oldRoute, httptest.NewRequest(http.MethodGet, "/api/existing-service-contract", nil))
	if oldRoute.Code != http.StatusAccepted || oldRoute.Body.String() != "original-base-route" {
		t.Fatalf("existing base handler was intercepted: %d %s", oldRoute.Code, oldRoute.Body.String())
	}
}

func TestMountFabricPreservesExistingAcceptanceCatalog(t *testing.T) {
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	mounted := MountFabric(base)
	res := httptest.NewRecorder()
	mounted.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center/acceptance", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "\"control_count\":170") {
		t.Fatalf("existing acceptance route did not reach Fabric: %d %s", res.Code, res.Body.String())
	}
}
