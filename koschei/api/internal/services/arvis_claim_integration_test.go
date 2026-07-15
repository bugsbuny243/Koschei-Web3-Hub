package services

import "testing"

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
