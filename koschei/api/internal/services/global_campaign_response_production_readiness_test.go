package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
)

func responseProductionReadinessEvidenceFixture() GlobalCampaignResponseProductionReadinessEvidence {
	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return GlobalCampaignResponseProductionReadinessEvidence{
		DeploymentRef:            "railway:koschei-api:production",
		DeployedRevision:         "3ad9035820b6b5109c438b8e997b2b6a7d453334",
		ForwarderRef:             "safe-forwarder:production-1",
		ForwarderArtifactSHA256:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProductionIdentityRef:    "runtime:koschei-api-production",
		EffectCollectorProducer:  "collector:response-effect-production",
		EffectCollectorPublicKey: base64.RawURLEncoding.EncodeToString(publicKey),
		AuthorizationSchema:      GlobalCampaignResponseAuthorizationSchemaVersion,
		EffectProofSchema:        GlobalCampaignEffectProofSchemaVersion,
	}
}

func TestGlobalCampaignResponseProductionReadinessCannotClaimReadyWithoutConcreteForwarder(t *testing.T) {
	status := EvaluateGlobalCampaignResponseProductionReadiness(responseProductionReadinessEvidenceFixture())
	if status.State != GlobalCampaignResponseProductionReadinessBlocked || status.ProductionContainmentReady || status.ProductionClaimAllowed || status.ConcreteForwarderLinked {
		t.Fatalf("production readiness escaped fail-closed boundary: %+v", status)
	}
	if len(status.Blockers) != 1 || status.Blockers[0] != GlobalCampaignResponseProductionBlockerForwarderMissing {
		t.Fatalf("expected only concrete forwarder blocker, got %v", status.Blockers)
	}
	if err := ValidateGlobalCampaignResponseProductionReadiness(status); err != nil {
		t.Fatalf("validate blocked readiness: %v", err)
	}
}

func TestGlobalCampaignResponseProductionReadinessMissingEvidenceIsDeterministic(t *testing.T) {
	first := EvaluateGlobalCampaignResponseProductionReadiness(GlobalCampaignResponseProductionReadinessEvidence{})
	second := EvaluateGlobalCampaignResponseProductionReadiness(GlobalCampaignResponseProductionReadinessEvidence{})
	if first.ReadinessHashSHA256 == "" || first.ReadinessHashSHA256 != second.ReadinessHashSHA256 {
		t.Fatalf("readiness hash is not deterministic: %q != %q", first.ReadinessHashSHA256, second.ReadinessHashSHA256)
	}
	if len(first.Blockers) != 8 {
		t.Fatalf("expected all eight blockers, got %d: %v", len(first.Blockers), first.Blockers)
	}
}

func TestGlobalCampaignResponseProductionReadinessRejectsContractDrift(t *testing.T) {
	evidence := responseProductionReadinessEvidenceFixture()
	evidence.AuthorizationSchema = "koschei.response-authorization.v2"
	evidence.EffectProofSchema = "koschei.response-effect-proof.v2"
	status := EvaluateGlobalCampaignResponseProductionReadiness(evidence)
	if status.ProductionClaimAllowed || status.ProductionContainmentReady {
		t.Fatalf("contract drift must block production claim: %+v", status)
	}
}

func TestGlobalCampaignResponseProductionReadinessRequiresIndependentCollectorIdentity(t *testing.T) {
	evidence := responseProductionReadinessEvidenceFixture()
	evidence.EffectCollectorProducer = evidence.ProductionIdentityRef
	status := EvaluateGlobalCampaignResponseProductionReadiness(evidence)
	seen := false
	for _, blocker := range status.Blockers {
		seen = seen || blocker == GlobalCampaignResponseProductionBlockerEffectCollector
	}
	if !seen || status.ProductionClaimAllowed {
		t.Fatalf("production runtime cannot self-attest effect readiness: %+v", status)
	}
}

func TestGlobalCampaignResponseProductionReadinessRejectsClaimTamper(t *testing.T) {
	status := EvaluateGlobalCampaignResponseProductionReadiness(responseProductionReadinessEvidenceFixture())
	status.State = GlobalCampaignResponseProductionReadinessReady
	status.ConcreteForwarderLinked = true
	status.ProductionContainmentReady = true
	status.ProductionClaimAllowed = true
	if err := ValidateGlobalCampaignResponseProductionReadiness(status); !errors.Is(err, ErrGlobalCampaignResponseProductionReadinessInvalid) {
		t.Fatalf("expected tampered claim to fail closed, got %v", err)
	}
}
