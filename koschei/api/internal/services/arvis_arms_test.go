package services

import (
	"testing"
	"time"
)

func TestUnavailableArvisArmIsNeverSigned(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	arm := unavailableArm("Liquidity Movement", ModuleLiquidityMovement, req, time.Now().UTC().Format(time.RFC3339), "missing reserve history")
	if arm.Signed {
		t.Fatal("unavailable arm must not be signed")
	}
	if arm.RiskIndex != 0 || arm.Grade != "-" || arm.Signature != "" {
		t.Fatalf("unexpected unavailable arm: %#v", arm)
	}
	if available, _ := arm.Signals["arm_evidence_available"].(bool); available {
		t.Fatal("unavailable arm reported evidence")
	}
}

func TestEvidenceArmIsSignedOnlyWithEvidenceSignals(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	signals := map[string]any{"real_onchain_evidence": true, "arm_evidence_available": true}
	arm := evidenceArm("Token Authority Scanner", ModuleTokenAuthorityScanner, req, 81, signals, []string{"parsed mint evidence"}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed || arm.Signature == "" {
		t.Fatalf("evidence arm should be signed: %#v", arm)
	}
	if arm.RiskIndex != 81 || arm.RiskLevel != "high" {
		t.Fatalf("unexpected risk result: %#v", arm)
	}
}

func TestFinalArvisArmUsesHighestVerifiedArm(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	low := evidenceArm("Program Relation Scan", ModuleProgramRelationScan, req, 20, map[string]any{"real_onchain_evidence": true}, []string{"owner evidence"}, generatedAt)
	high := evidenceArm("Holder Concentration", ModuleHolderConcentration, req, 72, map[string]any{"real_onchain_evidence": true}, []string{"holder evidence"}, generatedAt)
	missing := unavailableArm("MEV Shield", ModuleMEVShield, req, generatedAt, "transaction required")
	final := buildFinalArm(req, []SecurityRadarVerdict{low, high, missing}, generatedAt)
	if !final.Signed {
		t.Fatal("final arm should be signed when verified evidence exists")
	}
	if final.RiskIndex != 72 {
		t.Fatalf("expected highest verified risk 72, got %d", final.RiskIndex)
	}
	if winner, _ := final.Signals["winning_arm"].(string); winner != ModuleHolderConcentration {
		t.Fatalf("unexpected winning arm: %q", winner)
	}
}

func TestArvisBundleExposesFourteenArms(t *testing.T) {
	arms := make([]SecurityRadarVerdict, 14)
	bundle := SecurityRadarBundle{Metadata: map[string]any{"arvis_arms": arms}}
	got := ArvisArmsFromBundle(bundle)
	if len(got) != 14 {
		t.Fatalf("expected 14 arms, got %d", len(got))
	}
}
