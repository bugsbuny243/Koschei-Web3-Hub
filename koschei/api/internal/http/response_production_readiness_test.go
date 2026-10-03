package http

import (
	"crypto/ed25519"
	"encoding/base64"
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
	if status.State != services.GlobalCampaignResponseProductionReadinessBlocked ||
		status.ProductionContainmentReady || status.ProductionClaimAllowed || status.ConcreteForwarderLinked {
		t.Fatalf("runtime telemetry escaped fail-closed production boundary: %+v", status)
	}
	if len(status.Blockers) != 1 || status.Blockers[0] != services.GlobalCampaignResponseProductionBlockerForwarderMissing {
		t.Fatalf("expected only concrete forwarder blocker, got %v", status.Blockers)
	}
	if status.Evidence.DeployedRevision == "" || status.ReadinessHashSHA256 == "" {
		t.Fatalf("expected pinned revision and readiness hash: %+v", status)
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
	if len(status.Blockers) != 6 {
		t.Fatalf("expected six runtime/repository blockers with compile-time schemas pinned, got %d: %v", len(status.Blockers), status.Blockers)
	}
}
