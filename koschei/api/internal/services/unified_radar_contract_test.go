package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnifiedRuntimeContractAcceptsNoGradeVerdict(t *testing.T) {
	rawVerdict := EvaluateUnifiedRadarVerdict("MintOne", ActorDefenseRuleVerdict{}, UnifiedRadarBehaviorReport{})
	if rawVerdict.Grade != "-" {
		t.Fatalf("expected withheld grade, got %q", rawVerdict.Grade)
	}

	encoded, err := json.Marshal(rawVerdict)
	if err != nil {
		t.Fatalf("marshal runtime contract: %v", err)
	}
	var contract map[string]any
	if err := json.Unmarshal(encoded, &contract); err != nil {
		t.Fatalf("decode runtime contract: %v", err)
	}
	if contract["grade"] != "-" {
		t.Fatalf("contract grade=%v", contract["grade"])
	}
	if contract["signed"] != true {
		t.Fatalf("no-grade contract must be signed deterministic state: %v", contract["signed"])
	}
	if contract["rule_version"] != UnifiedRadarRulesetVersion {
		t.Fatalf("rule_version=%v", contract["rule_version"])
	}
	if _, exists := contract["risk_index"]; exists {
		t.Fatal("numeric risk_index leaked into signed contract")
	}
	if _, exists := contract["risk_level"]; exists {
		t.Fatal("risk_level leaked into numberless signed contract")
	}
	evidence, ok := contract["evidence"].([]any)
	if !ok || len(evidence) == 0 {
		t.Fatalf("contract evidence=%#v", contract["evidence"])
	}
	if rules, ok := contract["triggered_rules"].([]any); !ok || len(rules) != 0 {
		t.Fatalf("no-grade triggered_rules=%#v", contract["triggered_rules"])
	}
	decision, ok := contract["decision_path"].([]any)
	if !ok || len(decision) == 0 {
		t.Fatalf("decision_path=%#v", contract["decision_path"])
	}
	if signature, _ := contract["signature"].(string); !strings.HasPrefix(signature, "koschei-unified-contract:") {
		t.Fatalf("fallback contract signature=%q", signature)
	}
}

func TestFinalizeUnifiedRuntimeContractBindsTarget(t *testing.T) {
	verdict := EvaluateUnifiedRadarVerdict("MintOne", ActorDefenseRuleVerdict{}, UnifiedRadarBehaviorReport{})
	finalized := FinalizeUnifiedRadarVerdictContract("MintOne", verdict)
	if !finalized.Signed || finalized.Signature == "" {
		t.Fatalf("finalized verdict=%#v", finalized)
	}
	other := FinalizeUnifiedRadarVerdictContract("MintTwo", verdict)
	if finalized.Signature == other.Signature {
		t.Fatal("target-bound signatures must differ")
	}
}

func TestUnifiedRuntimeContractCarriesTriggeredRules(t *testing.T) {
	actor := ActorDefenseRuleVerdict{TriggeredRules: []ActorDefenseRuleHit{
		{RuleID: ActorRuleCompoundCreatorReuse, Title: "Creator reuse", Tier: "compounding", EvidenceStatus: "verified", GradeEffect: "compounding_input", Summary: "creator reused"},
		{RuleID: ActorRuleCompoundHolderReuse, Title: "Holder reuse", Tier: "compounding", EvidenceStatus: "observed", GradeEffect: "compounding_input", Summary: "holder reused"},
	}}
	verdict := FinalizeUnifiedRadarVerdictContract("MintOne", EvaluateUnifiedRadarVerdict("MintOne", actor, UnifiedRadarBehaviorReport{}))
	encoded, err := json.Marshal(verdict)
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]any
	if err := json.Unmarshal(encoded, &contract); err != nil {
		t.Fatal(err)
	}
	if contract["grade"] != "B" || contract["signed"] != true {
		t.Fatalf("contract=%s", encoded)
	}
	rules, ok := contract["triggered_rules"].([]any)
	if !ok || len(rules) != 2 {
		t.Fatalf("triggered_rules=%#v", contract["triggered_rules"])
	}
	evidence, ok := contract["evidence"].([]any)
	if !ok || len(evidence) != 2 {
		t.Fatalf("evidence=%#v", contract["evidence"])
	}
}
