package services

import (
	"strings"
	"testing"
	"time"
)

func TestFundingClusterHolderEvidenceSurvivesTransactionEnrichment(t *testing.T) {
	req := SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}
	base := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 72, map[string]any{
		"real_onchain_evidence":   true,
		"holder_cluster_analysis": HolderClusterAnalysis{Available: true, WalletsAnalyzed: 5},
	}, []string{"holder cluster"}, time.Now().UTC().Format(time.RFC3339))
	replacement := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 22, map[string]any{
		"real_onchain_evidence": true,
		"transaction_signature": "sig",
	}, []string{"initialization delta"}, time.Now().UTC().Format(time.RFC3339))
	arms := []SecurityRadarVerdict{base}
	replaceFundingClusterArmPreservingHolderEvidence(arms, replacement)
	if _, ok := arms[0].Signals["holder_cluster_analysis"]; !ok {
		t.Fatalf("holder cluster evidence was overwritten: %#v", arms[0])
	}
	if arms[0].RiskIndex != 0 || arms[0].Grade != "-" {
		t.Fatalf("preserved arm issued score/grade: %#v", arms[0])
	}
}

func TestFundingClusterTransactionFillsUnavailableBase(t *testing.T) {
	req := SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}
	base := unavailableArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, time.Now().UTC().Format(time.RFC3339), "missing")
	replacement := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 22, map[string]any{
		"real_onchain_evidence": true,
		"transaction_signature": "sig",
	}, []string{"initialization delta"}, time.Now().UTC().Format(time.RFC3339))
	arms := []SecurityRadarVerdict{base}
	replaceFundingClusterArmPreservingHolderEvidence(arms, replacement)
	if !arms[0].Signed || arms[0].Signals["transaction_signature"] != "sig" {
		t.Fatalf("verified transaction evidence did not fill unavailable base: %#v", arms[0])
	}
	if arms[0].RiskIndex != 0 || arms[0].Grade != "-" {
		t.Fatalf("transaction evidence issued score/grade: %#v", arms[0])
	}
}

func TestSniperTimingRejectsTruncatedLatestHundred(t *testing.T) {
	arm := buildSniperTimingArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, RecentSignatureCount: 100, SignatureWindowSeconds: 1,
		TargetSignatureHistoryExhausted: false, TargetSignatureTimingObserved: true,
	}, time.Now().UTC().Format(time.RFC3339))
	if arm.Signed || arm.RiskLevel != "unknown" {
		t.Fatalf("truncated recent window must not become sniper timing: %#v", arm)
	}
	if !strings.Contains(strings.Join(arm.Evidence, " "), "truncated") {
		t.Fatalf("expected truncated-window explanation: %#v", arm.Evidence)
	}
}

func TestSniperTimingAcceptsCompleteMintHistoryAsEvidenceOnly(t *testing.T) {
	arm := buildSniperTimingArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, RecentSignatureCount: 40, SignatureWindowSeconds: 8,
		TargetSignatureHistoryExhausted: true, TargetSignatureTimingObserved: true,
	}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed {
		t.Fatalf("complete observed history should be usable: %#v", arm)
	}
	if arm.RiskIndex != 0 || arm.Grade != "-" {
		t.Fatalf("sniper arm issued score/grade: %#v", arm)
	}
}

func TestMajorityEOAHolderProducesHolderEvidenceNotGrade(t *testing.T) {
	arm := buildHolderArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, IsTokenMint: true, LargestAccounts: 20,
		LargestHolderPct: 59, Top10HolderPct: 64,
		HolderRoles: HolderRoleAnalysis{Available: true},
	}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed {
		t.Fatalf("majority holder evidence should be signed: %#v", arm)
	}
	if arm.RiskIndex != 0 || arm.Grade != "-" || arm.RiskLevel != "evidence_only" {
		t.Fatalf("holder arm issued score/grade: %#v", arm)
	}
	if got := arm.Signals["largest_holder_percentage"]; got != 59 {
		t.Fatalf("largest-holder fact missing: %#v", arm.Signals)
	}
}
