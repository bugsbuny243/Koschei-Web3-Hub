package http

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"koschei/api/internal/services"
)

func responseReadinessRuntimeEnv() map[string]string {
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return map[string]string{
		responseDeploymentRefEnv:            "railway:koschei-api:production",
		"RAILWAY_GIT_COMMIT_SHA":            "f75d37b13d0407f2da7979c3e65cd588b95f52f5",
		responseForwarderRefEnv:             "safe-forwarder:production-1",
		responseForwarderArtifactSHAEnv:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		responseProductionIdentityRefEnv:    "runtime:koschei-api-production",
		responseEffectCollectorProducerEnv:  "collector:response-effect-production",
		responseEffectCollectorPublicKeyEnv: base64.RawURLEncoding.EncodeToString(publicKey),
	}
}

func responseReadinessGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestResponseProductionReadinessTelemetryCannotClaimProductionWithoutConcreteForwarder(t *testing.T) {
	status := responseProductionReadinessFrom(responseReadinessGetenv(responseReadinessRuntimeEnv()))
	if status.State != services.GlobalCampaignResponseProductionReadinessBlocked || status.ProductionContainmentReady || status.ProductionClaimAllowed || status.ConcreteForwarderLinked {
		t.Fatalf("runtime telemetry escaped fail-closed production boundary: %+v", status)
	}
	if len(status.Blockers) != 1 || status.Blockers[0] != services.GlobalCampaignResponseProductionBlockerForwarderMissing {
		t.Fatalf("expected only concrete forwarder blocker, got %v", status.Blockers)
	}
}

func TestResponseProductionReadinessTelemetryReportsUnpinnedDeployment(t *testing.T) {
	values := responseReadinessRuntimeEnv()
	delete(values, "RAILWAY_GIT_COMMIT_SHA")
	status := responseProductionReadinessFrom(responseReadinessGetenv(values))
	seenRevision := false
	seenForwarder := false
	for _, blocker := range status.Blockers {
		seenRevision = seenRevision || blocker == services.GlobalCampaignResponseProductionBlockerDeploymentUnpinned
		seenForwarder = seenForwarder || blocker == services.GlobalCampaignResponseProductionBlockerForwarderMissing
	}
	if !seenRevision || !seenForwarder || status.ProductionClaimAllowed {
		t.Fatalf("missing deployment blockers: %+v", status)
	}
}

func TestResponseProductionReadinessTelemetryNilEnvironmentFailsClosed(t *testing.T) {
	status := responseProductionReadinessFrom(nil)
	if status.State != services.GlobalCampaignResponseProductionReadinessBlocked || status.ProductionClaimAllowed {
		t.Fatalf("nil runtime environment must fail closed: %+v", status)
	}
}

func TestOwnerRouteMapIncludesResponseProductionReadinessTelemetry(t *testing.T) {
	for key, value := range responseReadinessRuntimeEnv() {
		t.Setenv(key, value)
	}
	request := httptest.NewRequest(http.MethodGet, "https://example.test/api/owner/route-map", nil)
	recorder := httptest.NewRecorder()
	ownerRouteMap(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("owner route map status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode owner route map: %v", err)
	}
	telemetry, ok := payload["response_production_readiness"].(map[string]any)
	if !ok {
		t.Fatalf("missing response production readiness telemetry: %#v", payload["response_production_readiness"])
	}
	if telemetry["state"] != services.GlobalCampaignResponseProductionReadinessBlocked || telemetry["production_claim_allowed"] != false {
		t.Fatalf("owner telemetry must expose blocked production claim: %#v", telemetry)
	}
}
