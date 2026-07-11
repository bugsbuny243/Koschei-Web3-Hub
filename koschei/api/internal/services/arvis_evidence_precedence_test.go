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
	if arms[0].RiskIndex != 72 {
		t.Fatalf("holder cluster was overwritten: %#v", arms[0])
	}
}

func TestFundingClusterTransactionFillsUnavailableBase(t *testing.T) {
	req := SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}
	base := unavailableArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, time.Now().UTC().Format(time.RFC3339), "missing")
	replacement := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 22, map[string]any{"real_onchain_evidence": true}, []string{"initialization delta"}, time.Now().UTC().Format(time.RFC3339))
	arms := []SecurityRadarVerdict{base}
	replaceFundingClusterArmPreservingHolderEvidence(arms, replacement)
	if !arms[0].Signed || arms[0].RiskIndex != 22 {
		t.Fatalf("verified transaction evidence did not fill unavailable base: %#v", arms[0])
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

func TestSniperTimingAcceptsCompleteMintHistory(t *testing.T) {
	arm := buildSniperTimingArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, RecentSignatureCount: 40, SignatureWindowSeconds: 8,
		TargetSignatureHistoryExhausted: true, TargetSignatureTimingObserved: true,
	}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed {
		t.Fatalf("complete observed history should be usable: %#v", arm)
	}
}

func TestMajorityEOAHolderIsHighRisk(t *testing.T) {
	arm := buildHolderArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, IsTokenMint: true, LargestAccounts: 20,
		LargestHolderPct: 59, Top10HolderPct: 64,
	}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed || arm.RiskIndex < 65 || arm.RiskLevel != "high" {
		t.Fatalf("majority holder must not be LOW: %#v", arm)
	}
}
