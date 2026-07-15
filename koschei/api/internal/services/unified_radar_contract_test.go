package services

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnifiedRadarDashGradeMarshalsAsScorelessSignedContract(t *testing.T) {
	verdict := UnifiedRadarVerdict{
		Grade: "-", Verdict: "no_grade_trigger", RulesetVersion: UnifiedRadarRulesetVersion,
		ActorRuleset: ActorDefenseRulesetVersion,
		DecisionPath: []string{"No grade-changing rule was satisfied; absence of evidence is not an A grade."},
		GeneratedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(verdict)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["grade"] != "-" || payload["signed"] != true {
		t.Fatalf("dash verdict contract mismatch: %s", raw)
	}
	if payload["rule_version"] != UnifiedRadarRulesetVersion {
		t.Fatalf("rule_version missing: %s", raw)
	}
	if _, exists := payload["risk_index"]; exists {
		t.Fatalf("risk_index must not be emitted: %s", raw)
	}
	if _, exists := payload["risk_level"]; exists {
		t.Fatalf("risk_level must not be emitted: %s", raw)
	}
	if evidence, ok := payload["evidence"].([]any); !ok || len(evidence) == 0 {
		t.Fatalf("evidence missing: %s", raw)
	}
	if decision, ok := payload["decision_path"].([]any); !ok || len(decision) == 0 {
		t.Fatalf("decision_path missing: %s", raw)
	}
}
