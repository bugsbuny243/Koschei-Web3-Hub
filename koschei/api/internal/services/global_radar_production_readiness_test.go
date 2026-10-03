package services

import (
	"testing"
	"time"
)

func productionReadinessFixture() (GlobalRadarProductionReadinessPolicy, GlobalRadarProductionReadinessEvidence, time.Time) {
	evaluatedAt := time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)
	policy := GlobalRadarProductionReadinessPolicyV1()
	evidence := GlobalRadarProductionReadinessEvidence{
		ExpectedRevision:                    "deploy:abc123",
		DeployedRevision:                    "deploy:abc123",
		WindowStart:                         evaluatedAt.Add(-25 * time.Hour),
		WindowEnd:                           evaluatedAt.Add(-time.Minute),
		Networks:                            []string{"solana", "ethereum", "bitcoin"},
		CoverageComplete:                    true,
		BlindGapCount:                       0,
		OpenIngestGapCount:                  0,
		UnresolvedProviderDisagreementCount: 0,
		MaxIngestLagSeconds:                 30,
		MaxBacklogDepth:                     100,
		MTTDSeconds:                         60,
		MTTRSeconds:                         600,
		EventsProcessed:                     1000000,
		TotalCostMicrounits:                 100000000,
		ArchiveRestoreVerified:              true,
		ArchiveRestoreEvidenceSHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ReorgRecoveryVerified:               true,
		ReorgRecoveryEvidenceSHA256:         "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ResponseCapabilityClaimed:           true,
		VerifiedEffectProofCount:            1,
		EvidenceRefs:                        []string{"evidence:coverage", "evidence:latency", "evidence:restore"},
		ObservedAt:                          evaluatedAt.Add(-time.Minute),
	}
	return policy, evidence, evaluatedAt
}

func TestGlobalRadarProductionReadinessReadyOnlyWithCompleteEvidence(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	readiness, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build readiness: %v", err)
	}
	if !readiness.Ready || readiness.Status != GlobalRadarProductionReadinessReady || len(readiness.FailedGates) != 0 || len(readiness.MissingEvidence) != 0 {
		t.Fatalf("expected READY, got status=%s ready=%v failed=%v missing=%v", readiness.Status, readiness.Ready, readiness.FailedGates, readiness.MissingEvidence)
	}
	if readiness.VerdictAuthority || readiness.ContainmentAuthority || readiness.ExecutionAuthority || readiness.ReadinessHashSHA256 == "" {
		t.Fatalf("readiness gate must remain non-authoritative and hashed: %#v", readiness)
	}
}

func TestGlobalRadarProductionReadinessMissingProofIsUnknown(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	evidence.ArchiveRestoreVerified = false
	evidence.ArchiveRestoreEvidenceSHA256 = ""
	readiness, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build readiness: %v", err)
	}
	if readiness.Status != GlobalRadarProductionReadinessUnknown || readiness.Ready || len(readiness.MissingEvidence) == 0 {
		t.Fatalf("missing proof must be UNKNOWN: %#v", readiness)
	}
}

func TestGlobalRadarProductionReadinessRevisionMismatchIsNotReady(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	evidence.DeployedRevision = "deploy:other"
	readiness, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build readiness: %v", err)
	}
	if readiness.Status != GlobalRadarProductionReadinessNotReady || readiness.Ready {
		t.Fatalf("revision mismatch must be NOT_READY: %#v", readiness)
	}
}

func TestGlobalRadarProductionReadinessBlindGapIsNotReady(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	evidence.BlindGapCount = 1
	readiness, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build readiness: %v", err)
	}
	if readiness.Status != GlobalRadarProductionReadinessNotReady || readiness.Ready {
		t.Fatalf("blind gap must be NOT_READY: %#v", readiness)
	}
}

func TestGlobalRadarProductionReadinessStaleEvidenceIsNotReady(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	evidence.ObservedAt = evaluatedAt.Add(-time.Duration(policy.MaxEvidenceAgeSeconds+1) * time.Second)
	readiness, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build readiness: %v", err)
	}
	if readiness.Status != GlobalRadarProductionReadinessNotReady || readiness.Ready {
		t.Fatalf("stale evidence must be NOT_READY: %#v", readiness)
	}
}

func TestGlobalRadarProductionReadinessResponseClaimNeedsVerifiedEffect(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	evidence.VerifiedEffectProofCount = 0
	readiness, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build readiness: %v", err)
	}
	if readiness.Status != GlobalRadarProductionReadinessNotReady || readiness.Ready {
		t.Fatalf("response claim without effect proof must be NOT_READY: %#v", readiness)
	}
}

func TestGlobalRadarProductionReadinessCanonicalOrderIsDeterministic(t *testing.T) {
	policy, evidence, evaluatedAt := productionReadinessFixture()
	first, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build first readiness: %v", err)
	}
	evidence.Networks = []string{"bitcoin", "solana", "ethereum"}
	evidence.EvidenceRefs = []string{"evidence:restore", "evidence:coverage", "evidence:latency"}
	second, err := BuildGlobalRadarProductionReadiness(policy, evidence, evaluatedAt)
	if err != nil {
		t.Fatalf("build second readiness: %v", err)
	}
	if first.ReadinessHashSHA256 != second.ReadinessHashSHA256 {
		t.Fatalf("input order changed readiness hash: %s != %s", first.ReadinessHashSHA256, second.ReadinessHashSHA256)
	}
}
