package services

import "testing"

func TestEvidenceGateRejectsUnsignedOutputWithoutLiveEvidence(t *testing.T) {
	bundle := SecurityRadarBundle{
		PumpSybilRadar: SecurityRadarVerdict{
			ModuleID:  ModulePumpSybilRadar,
			RiskIndex: 28,
			RiskLevel: "low",
			Grade:     "A",
			Signed:    true,
			Signature: "fake-signature",
			Signals:   map[string]any{"real_onchain_evidence": false},
		},
		RaydiumPoolGuardian: SecurityRadarVerdict{
			ModuleID:  ModuleRaydiumPoolGuardian,
			RiskIndex: 28,
			RiskLevel: "low",
			Grade:     "A",
			Signed:    true,
			Signature: "fake-signature-2",
			Signals:   map[string]any{"real_onchain_evidence": false},
		},
	}

	gated := EvidenceBackedSecurityRadarBundle(bundle)
	final := EvidenceBackedFinalSecurityRadarVerdict(gated)

	if SecurityRadarHasLiveEvidence(gated) {
		t.Fatal("expected no live evidence")
	}
	if final.Signed {
		t.Fatal("verdict without live evidence must not be signed")
	}
	if final.RiskIndex != 0 || final.Grade != "-" || final.Signature != "" {
		t.Fatalf("unexpected insufficient-evidence final verdict: %#v", final)
	}
	if final.Recommendation != "insufficient_evidence" {
		t.Fatalf("unexpected recommendation: %s", final.Recommendation)
	}
}

func TestEvidenceGateNeverPromotesLegacyArmWithVerifiedLiveOutput(t *testing.T) {
	bundle := SecurityRadarBundle{
		PumpSybilRadar: SecurityRadarVerdict{
			ModuleID:       ModulePumpSybilRadar,
			RiskIndex:      46,
			RiskLevel:      "medium",
			Grade:          "B",
			Verdict:        "Verified observation",
			Recommendation: "watch",
			Signed:         true,
			Signature:      "verified-signature",
			Signals:        map[string]any{"real_onchain_evidence": true},
		},
		RaydiumPoolGuardian: SecurityRadarVerdict{
			ModuleID:       ModuleRaydiumPoolGuardian,
			RiskIndex:      20,
			RiskLevel:      "low",
			Grade:          "A",
			Recommendation: "safe_to_monitor",
			Signed:         true,
			Signature:      "verified-signature-2",
			Signals:        map[string]any{"real_onchain_evidence": true},
		},
	}

	gated := EvidenceBackedSecurityRadarBundle(bundle)
	final := EvidenceBackedFinalSecurityRadarVerdict(gated)

	if !SecurityRadarHasLiveEvidence(gated) {
		t.Fatal("expected live evidence")
	}
	if final.Signed || final.RiskIndex != 0 || final.Grade != "-" || final.Signature != "" ||
		final.Recommendation != "evaluate_unified_rules" {
		t.Fatalf("legacy arm promoted into canonical final authority: %#v", final)
	}
}

func TestLegacyFinalCannotBorrowAnotherArmsEvidenceOrFakeFinalSignature(t *testing.T) {
	bundle := SecurityRadarBundle{
		PumpSybilRadar: SecurityRadarVerdict{
			ModuleID: ModulePumpSybilRadar,
			Grade: "B", RiskIndex: 46, RiskLevel: "medium",
			Signed: true, Signature: "forged-legacy-pump",
			Signals: map[string]any{"real_onchain_evidence": true},
		},
		RaydiumPoolGuardian: SecurityRadarVerdict{
			ModuleID: ModuleRaydiumPoolGuardian,
			Grade: "A", RiskIndex: 99, RiskLevel: "high",
			Signed: true, Signature: "forged-legacy-raydium",
			Signals: map[string]any{"real_onchain_evidence": false},
		},
		Metadata: map[string]any{
			"arvis_arms": []SecurityRadarVerdict{
				{ModuleID: ModulePumpSybilRadar, Signed: true,
					Signals: map[string]any{"real_onchain_evidence": true}},
				{ModuleID: ModuleFinalVerdictEngine,
					Grade: "A", RiskIndex: 1, RiskLevel: "low",
					Signed: true, Signature: "forged-legacy-final",
					Signals: map[string]any{"verified_evidence": true}},
			},
		},
	}
	if !SecurityRadarHasLiveEvidence(bundle) {
		t.Fatal("fixture requires a verified non-final evidence arm")
	}
	final := EvidenceBackedLegacyFinalFromGatedBundle(bundle)
	if final.Signed || final.Signature != "" || final.Grade != "-" || final.RiskIndex != 0 ||
		final.RiskLevel != "unknown" || final.Recommendation != "evaluate_unified_rules" {
		t.Fatalf("legacy final borrowed a score or fabricated signature: %#v", final)
	}
}

func TestLegacyFinalCannotTreatOnlyFakeFinalArmAsEvidence(t *testing.T) {
	bundle := SecurityRadarBundle{
		Metadata: map[string]any{"arvis_arms": []SecurityRadarVerdict{{
			ModuleID: ModuleFinalVerdictEngine, Grade: "A", RiskIndex: 2,
			Signed: true, Signature: "fake", EvidenceVerified: true,
		}}},
	}
	if SecurityRadarHasLiveEvidence(bundle) {
		t.Fatal("a final arm cannot establish evidence authority")
	}
	got := EvidenceBackedLegacyFinalFromGatedBundle(bundle)
	if got.Signed || got.Signature != "" || got.RiskLevel != "unknown" ||
		got.Grade != "-" || got.Recommendation != "insufficient_evidence" {
		t.Fatalf("forged final-only bundle escaped WITHHOLD boundary: %#v", got)
	}
}
