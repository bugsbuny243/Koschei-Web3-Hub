package services

import (
	"errors"
	"testing"
	"time"
)

func TestProjectSecurityRadarBundleToGlobalRadarProjectsVerifiedArmsOnly(t *testing.T) {
	generated := time.Date(2026, 10, 4, 7, 0, 0, 0, time.UTC)
	target := "11111111111111111111111111111111"
	verified := SecurityRadarVerdict{
		Module: "Verified arm", ModuleID: "verified_arm", Target: target, Network: "solana-mainnet",
		Grade: "C", RiskIndex: 42, RiskLevel: "medium", Verdict: "review", Recommendation: "monitor",
		Evidence: []string{"tx:abc", "account:def"}, GeneratedAt: generated.Add(-time.Minute).Format(time.RFC3339Nano),
		RuleVersion: "rule-v1", EvidenceVerified: true, Signed: true, Signature: "sig", SignatureAlgorithm: "ed25519",
		KeyID: "key-1", PayloadHash: "sha256:payload", Digest: "sha256:digest",
	}
	unverified := verified
	unverified.ModuleID = "unverified_arm"
	unverified.EvidenceVerified = false
	unverified.Signals = map[string]any{}
	bundle := SecurityRadarBundle{
		Target: target, Network: "solana-mainnet",
		Metadata: map[string]any{"arvis_arms": []SecurityRadarVerdict{unverified, verified}},
	}

	snapshot, err := ProjectSecurityRadarBundleToGlobalRadar(bundle, generated)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != GlobalRadarSnapshotSchemaVersion {
		t.Fatalf("schema=%q", snapshot.SchemaVersion)
	}
	if len(snapshot.Observations) != 1 {
		t.Fatalf("observations=%d want 1", len(snapshot.Observations))
	}
	observation := snapshot.Observations[0]
	if observation.Subject.Raw != target || observation.Subject.ChainFamily != IntelligenceChainFamilySolana {
		t.Fatalf("unexpected subject: %+v", observation.Subject)
	}
	if observation.Evidence.Status != IntelligenceEvidenceVerified || observation.ObservationKind != GlobalRadarObservationThreat {
		t.Fatalf("unexpected observation: %+v", observation)
	}
	if observation.Evidence.Attributes["authority_boundary"] != "arvis_evidence_projection_only" {
		t.Fatalf("authority boundary missing: %+v", observation.Evidence.Attributes)
	}
	if snapshot.Coverage.VerifiedEvidenceCount != 1 || snapshot.Coverage.RiskScoreProduced {
		t.Fatalf("unexpected coverage: %+v", snapshot.Coverage)
	}
}

func TestProjectSecurityRadarBundleToGlobalRadarFailsClosedWithoutVerifiedEvidence(t *testing.T) {
	target := "11111111111111111111111111111111"
	bundle := SecurityRadarBundle{
		Target: target, Network: "solana-mainnet",
		Metadata: map[string]any{"arvis_arms": []SecurityRadarVerdict{{
			ModuleID: "arm", Target: target, Network: "solana-mainnet", GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}}},
	}
	_, err := ProjectSecurityRadarBundleToGlobalRadar(bundle, time.Now().UTC())
	if !errors.Is(err, ErrGlobalRadarARVISBundleEmpty) {
		t.Fatalf("err=%v want no verified evidence", err)
	}
}
