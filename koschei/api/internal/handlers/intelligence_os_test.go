package handlers

import "testing"

func TestRiskResultUsesPreliminaryStructuredLanguage(t *testing.T) {
	score, severity, flags, evidence, recommendations := riskResult(riskInput{Target: "0xabc", TargetType: "contract"})
	if score < 1 || severity == "" || len(flags) == 0 || len(evidence) == 0 || len(recommendations) == 0 {
		t.Fatalf("expected structured preliminary result, got score=%d severity=%q", score, severity)
	}
}

func TestGrantContentIsDraftOnly(t *testing.T) {
	draft := grantContent("Optimism", "public goods")
	text, _ := draft["generated_text"].(string)
	if text == "" || draft["ecosystem"] != "Optimism" {
		t.Fatalf("expected an ecosystem-specific copy-ready draft")
	}
}
