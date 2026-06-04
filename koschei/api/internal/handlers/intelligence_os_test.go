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

func TestFundingAssistantDraftContainsCopyReadySections(t *testing.T) {
	draft := fundingAssistantDraft(fundingAssistantInput{
		ProjectName:      "Builder Tool",
		Ecosystem:        "Base",
		ProjectCategory:  "Developer tooling",
		ShortDescription: "A no-custody builder workflow",
		MilestoneCount:   2,
	})
	milestones, ok := draft["milestones"].([]map[string]string)
	if !ok || len(milestones) != 2 || draft["copy_ready_application_text"] == "" {
		t.Fatalf("expected a copy-ready funding draft with two milestones")
	}
}
