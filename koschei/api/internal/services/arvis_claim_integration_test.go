package services

import (
	"strings"
	"testing"
)

func TestClaimURLFlowsThroughEvidenceGateWithoutArmGrade(t *testing.T) {
	req := SecurityRadarRequest{
		Target:  "http://192.0.2.7:8080/airdrop/claim?seedphrase=alpha&approve_transaction=1",
		Network: "solana-mainnet",
		Mode:    "manual_dashboard_check",
	}
	analysis := AnalyzeArvisRadars(req)
	bundle := EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := ArvisArmsFromBundle(bundle)
	final := ArvisFinalFromBundle(bundle)

	if len(arms) != 14 {
		t.Fatalf("expected 14 ARVIS arms, got %d", len(arms))
	}
	if !SecurityRadarHasLiveEvidence(bundle) {
		t.Fatal("verified claim-surface evidence should pass the evidence gate")
	}
	if final.Signed || final.RiskIndex != 0 || final.Grade != "-" {
		t.Fatalf("ARVIS compatibility final issued score or grade: %#v", final)
	}
	verifiedClaimArms := 0
	for _, arm := range arms {
		if arm.ModuleID != ModuleWalletlessClaimShield && arm.ModuleID != ModuleClaimSurfaceRisk {
			continue
		}
		if !SecurityRadarVerdictHasVerifiedEvidence(arm) {
			t.Fatalf("claim arm was not verified: %#v", arm)
		}
		if arm.RiskIndex != 0 || arm.Grade != "-" {
			t.Fatalf("claim arm issued score/grade: %#v", arm)
		}
		if onchain, _ := arm.Signals["real_onchain_evidence"].(bool); onchain {
			t.Fatal("claim surface arm incorrectly labeled as on-chain")
		}
		verifiedClaimArms++
	}
	if verifiedClaimArms != 2 {
		t.Fatalf("expected 2 verified claim arms, got %d", verifiedClaimArms)
	}
	if bundle.Provider != "url_parser" {
		t.Fatalf("expected url_parser provider, got %q", bundle.Provider)
	}
	if source, _ := bundle.Metadata["final_verdict_source"].(string); source != "EvaluateUnifiedRadarVerdict" {
		t.Fatalf("unexpected final source: %q", source)
	}
}

func TestMintOnlyPYUSDClaimArmsAreNotApplicableWithoutClaimURL(t *testing.T) {
	req := SecurityRadarRequest{
		Target:  "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo",
		Network: "solana-mainnet",
		Mode:    "manual_dashboard_check",
	}
	analysis := AnalyzeArvisRadars(req)
	for _, arm := range ArvisArmsFromBundle(analysis.Bundle) {
		if arm.ModuleID != ModuleWalletlessClaimShield && arm.ModuleID != ModuleClaimSurfaceRisk {
			continue
		}
		if arm.Recommendation != "not_applicable" || arvisArmExecutionStatus(arm) != ArvisExecutionNotApplicable ||
			!strings.Contains(arm.Verdict, "No claim URL was supplied") || arvisSignalBool(arm.Signals, "collector_attempted") {
			t.Fatalf("raw claim arm incorrectly reports a failed URL analysis: %#v", arm)
		}
	}
	coverage := BuildArvisInvestigationCoverage(ArvisArmsFromBundle(analysis.Bundle))
	found := 0
	for _, arm := range coverage.Arms {
		if arm.ModuleID != ModuleWalletlessClaimShield && arm.ModuleID != ModuleClaimSurfaceRisk {
			continue
		}
		found++
		if arm.ExecutionStatus != ArvisExecutionNotApplicable || arm.Applicable || arm.Attempted {
			t.Fatalf("claim URL not supplied; arm must be not_applicable, not data unavailable: %#v", arm)
		}
		if arm.Subchecks.NoEvidenceReason != "not_applicable" ||
			arm.Subchecks.RequiredChecksStatus != "not_applicable" || arm.Subchecks.Ran {
			t.Fatalf("claim subchecks must not imply an attempted scan: %#v", arm.Subchecks)
		}
	}
	if found != 2 || coverage.NotApplicable < 2 {
		t.Fatalf("both claim URL arms must be not applicable to mint-only evidence: %#v", coverage)
	}
}

func TestClaimApplicabilityRetainsExplicitCollectionFailuresAndURLObservations(t *testing.T) {
	req := SecurityRadarRequest{Target: "https://example.org/claim", Network: "solana-mainnet"}
	analysis := AnalyzeArvisRadars(req)
	coverage := BuildArvisInvestigationCoverage(ArvisArmsFromBundle(analysis.Bundle))
	found := 0
	for _, arm := range coverage.Arms {
		if arm.ModuleID != ModuleWalletlessClaimShield && arm.ModuleID != ModuleClaimSurfaceRisk {
			continue
		}
		found++
		if arm.ExecutionStatus != ArvisExecutionCompleted || !arm.EvidenceVerified {
			t.Fatalf("valid URL evidence must remain completed: %#v", arm)
		}
	}
	if found != 2 {
		t.Fatalf("expected both URL collectors; got %d", found)
	}
	// Explicitly attempted-but-incomplete collection must never be disguised
	// as not_applicable merely because this is a claim module.
	partial := evidencePendingArm("Claim Surface Risk", ModuleClaimSurfaceRisk, req, "now", "URL read failed", "claim_url_source_unavailable")
	partial.Signals["collector_attempted"] = true
	if status := arvisArmExecutionStatus(partial); status != ArvisExecutionEvidencePending {
		t.Fatalf("attempted claim URL inspection failed but became %q", status)
	}
}
