package handlers

import (
	"encoding/json"
	"testing"

	"koschei/api/internal/services"
)

func TestNumericScorePolicyIsConsistentOnTokenScan(t *testing.T) {
	c := services.UnrequestedArvisCoverage()
	response := tokenScanResponse{
		Mint: "FixtureMint111", Network: "solana-mainnet",
		Score: 80, TokenHeuristicPenaltyScore: 20,
		NotAFinalRiskScore: true, FinalScoreDisabledByPolicy: true,
		Coverage: c, ArmCount: c.ArmCount(), Arms: []services.SecurityRadarVerdict{},
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, exists := decoded["score"]; exists {
		t.Fatalf("deprecated numeric final-looking score must not be on wire: %s", raw)
	}
	if decoded["token_heuristic_penalty_score"] != float64(20) ||
		decoded["not_a_final_risk_score"] != true ||
		decoded["final_score_disabled_by_policy"] != true {
		t.Fatalf("penalty score must be explicitly identified as non-final: %s", raw)
	}
	if _, ok := decoded["coverage"].(map[string]any); !ok {
		t.Fatalf("token-scan coverage missing: %s", raw)
	}
	if _, ok := decoded["arm_count"].(map[string]any); !ok {
		t.Fatalf("token-scan arm_count missing: %s", raw)
	}
	if event, ok := decoded["security_evidence"].(map[string]any); !ok {
		t.Fatalf("security evidence missing: %s", raw)
	} else if _, legacy := event["legacy_observation"]; legacy {
		t.Fatalf("legacy raw score must not leak inside security evidence: %s", raw)
	}
}

func TestRadarCustomerEnvelopeCarriesArmDenominator(t *testing.T) {
	assembly := unifiedInvestigationAssembly{
		Report: map[string]any{"ok": true, "schema_version": unifiedInvestigationSchemaVersion},
		Core: holderIntelligenceCoreResult{
			Request: services.SecurityRadarRequest{Target: "Mint111", Network: "solana-mainnet"},
			Arms: []services.SecurityRadarVerdict{
				{ModuleID: services.ModuleWalletlessClaimShield, Module: "Walletless Claim Shield",
					Signals: map[string]any{"execution_status": services.ArvisExecutionEvidencePending,
						"reason_code": "collector_evidence_not_completed"}},
			},
		},
	}
	envelope := customerInvestigationEnvelope(assembly, false)
	coverage, ok := envelope["coverage"].(services.ArvisCoverageSummary)
	if !ok || coverage.ArmsTotal != 1 || coverage.ArmsWithheld != 1 {
		t.Fatalf("professional response missing coverage denominator: %#v", envelope["coverage"])
	}
	if count, ok := envelope["arm_count"].(services.ArvisArmCountSummary); !ok || count.ArmsTotal != 1 {
		t.Fatalf("professional response missing arm_count: %#v", envelope["arm_count"])
	}
}
