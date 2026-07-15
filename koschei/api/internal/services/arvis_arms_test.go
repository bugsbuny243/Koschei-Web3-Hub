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

func TestEvidenceArmSignsEvidenceWithoutIssuingGrade(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	signals := map[string]any{"real_onchain_evidence": true, "arm_evidence_available": true}
	arm := evidenceArm("Token Authority Scanner", ModuleTokenAuthorityScanner, req, 81, signals, []string{"parsed mint evidence"}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed || arm.Signature == "" {
		t.Fatalf("evidence arm should be signed: %#v", arm)
	}
	if arm.RiskIndex != 0 || arm.Grade != "-" || arm.RiskLevel != "evidence_only" {
		t.Fatalf("evidence arm issued a score or grade: %#v", arm)
	}
	if disabled, _ := arm.Signals["numeric_score_disabled"].(bool); !disabled {
		t.Fatalf("numeric-score policy missing: %#v", arm.Signals)
	}
}

func TestFinalArvisArmIsOnlyCompatibilityAdapter(t *testing.T) {
	req := SecurityRadarRequest{Target: "target", Network: "solana-mainnet"}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	first := evidenceArm("Program Relation Scan", ModuleProgramRelationScan, req, 20, map[string]any{"real_onchain_evidence": true}, []string{"owner evidence"}, generatedAt)
	second := evidenceArm("Holder Concentration", ModuleHolderConcentration, req, 72, map[string]any{"real_onchain_evidence": true}, []string{"holder evidence"}, generatedAt)
	final := buildFinalArm(req, []SecurityRadarVerdict{first, second}, generatedAt)
	if final.Signed || final.Grade != "-" || final.RiskIndex != 0 {
		t.Fatalf("compatibility final issued a verdict: %#v", final)
	}
	if source, _ := final.Signals["verdict_source"].(string); source != "EvaluateUnifiedRadarVerdict" {
		t.Fatalf("unexpected verdict source: %q", source)
	}
	if _, exists := final.Signals["winning_arm"]; exists {
		t.Fatalf("compatibility final selected a winning arm: %#v", final.Signals)
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
