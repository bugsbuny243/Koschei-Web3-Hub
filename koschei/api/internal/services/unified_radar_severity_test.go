package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnifiedVerdictCapsSevereHolderLiquidityExitAtC(t *testing.T) {
	behavior := UnifiedRadarBehaviorReport{Signals: []UnifiedRadarSignal{
		{
			RuleID:         UnifiedRuleHolderLiquidityPressure,
			Title:          "Dominant-holder position / liquidity depth",
			EvidenceStatus: "observed",
			Triggered:      true,
			GradeEffect:    "compounding_input",
			Metrics:        map[string]any{"position_liquidity_ratio": 49.17},
			Summary:        "Dominant-holder reference position equals 49.17x reported liquidity.",
		},
		{
			RuleID:         UnifiedRuleDominantHolderFirstExit,
			Title:          "Dominant-holder first observed exit",
			EvidenceStatus: "verified",
			Triggered:      true,
			GradeEffect:    "compounding_input",
			Metrics:        map[string]any{"amount": 1000.0, "destination": "PoolOne"},
			Summary:        "A transaction-backed dominant-holder exit was observed.",
		},
	}}

	verdict := EvaluateUnifiedRadarVerdict("MintOne", ActorDefenseRuleVerdict{}, behavior)
	if verdict.Grade != "C" || verdict.Verdict != "severe_compounding_rule" || verdict.Signed || verdict.Digest == "" {
		t.Fatalf("severity-aware verdict=%#v", verdict)
	}
	decision := strings.Join(verdict.DecisionPath, " ")
	if !strings.Contains(decision, UnifiedRuleHolderLiquidityPressure) ||
		!strings.Contains(decision, UnifiedRuleDominantHolderFirstExit) ||
		!strings.Contains(decision, "49.17x") {
		t.Fatalf("decision path does not expose rule facts: %q", decision)
	}
}

func TestUnifiedRadarSeveritySurvivesFinalizationAndJSON(t *testing.T) {
	hits := []ActorDefenseRuleHit{
		{
			RuleID: UnifiedRuleHolderLiquidityPressure, Tier: "compounding",
			EvidenceStatus: "observed", GradeEffect: "compounding_input",
			Facts: map[string]any{"metrics": map[string]any{"position_liquidity_ratio": 49.17}},
		},
		{
			RuleID: UnifiedRuleDominantHolderFirstExit, Tier: "compounding",
			EvidenceStatus: "verified", GradeEffect: "compounding_input",
			Facts: map[string]any{"metrics": map[string]any{"amount": 1000.0}},
		},
	}
	initial := UnifiedRadarVerdict{
		Grade: "C", Verdict: "severe_compounding_rule",
		TriggeredRules: hits,
	}
	final := FinalizeUnifiedRadarVerdictContractForNetwork("MintSevere", "solana-mainnet", initial)
	if final.Grade != "C" || final.Verdict != "severe_compounding_rule" {
		t.Fatalf("evidence-backed C was silently downgraded during finalization: %#v", final)
	}
	encoded, err := json.Marshal(final)
	if err != nil {
		t.Fatal(err)
	}
	var public struct {
		Grade   string `json:"grade"`
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal(encoded, &public); err != nil {
		t.Fatal(err)
	}
	if public.Grade != "C" || public.Verdict != "severe_compounding_rule" {
		t.Fatalf("public JSON weakened severe grade: %s", encoded)
	}
	if unifiedRadarContractSevereCompounding(hits, true) {
		t.Fatal("strict mode must not grade observed pressure as verified")
	}
	hits[0].EvidenceStatus = "verified"
	if !unifiedRadarContractSevereCompounding(hits, true) {
		t.Fatal("two verified severe trigger facts must pass strict eligibility")
	}
}

func TestUnifiedRadarSeverityCannotBeClaimedWithoutThresholdEvidence(t *testing.T) {
	hits := []ActorDefenseRuleHit{
		{
			RuleID: UnifiedRuleHolderLiquidityPressure, Tier: "compounding",
			EvidenceStatus: "verified", GradeEffect: "compounding_input",
			Facts: map[string]any{"metrics": map[string]any{"position_liquidity_ratio": 9.99}},
		},
		{
			RuleID: UnifiedRuleDominantHolderFirstExit, Tier: "compounding",
			EvidenceStatus: "verified", GradeEffect: "compounding_input",
		},
	}
	final := FinalizeUnifiedRadarVerdictContractForNetwork("MintUnproven", "solana-mainnet", UnifiedRadarVerdict{
		Grade: "C", Verdict: "severe_compounding_rule", TriggeredRules: hits,
	})
	if final.Grade != "B" || final.Verdict != "compounding_rule" {
		t.Fatalf("a claimed C cannot bypass missing threshold evidence: %#v", final)
	}
}
