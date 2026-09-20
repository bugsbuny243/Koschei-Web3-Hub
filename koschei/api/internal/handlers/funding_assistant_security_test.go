package handlers

import "testing"

func TestFundingAssistantDraftCapsMilestones(t *testing.T) {
	t.Parallel()

	draft := fundingAssistantDraft(fundingAssistantInput{
		ProjectName:      "Koschei",
		ShortDescription: "Security tooling",
		MilestoneCount:   maxFundingAssistantMilestones + 1000,
	})
	milestones, ok := draft["milestones"].([]map[string]string)
	if !ok {
		t.Fatalf("unexpected milestones type %T", draft["milestones"])
	}
	if got := len(milestones); got != maxFundingAssistantMilestones {
		t.Fatalf("milestone count = %d, want %d", got, maxFundingAssistantMilestones)
	}
}

func TestFundingAssistantDraftDefaultMilestones(t *testing.T) {
	t.Parallel()

	draft := fundingAssistantDraft(fundingAssistantInput{
		ProjectName:      "Koschei",
		ShortDescription: "Security tooling",
	})
	milestones := draft["milestones"].([]map[string]string)
	if got := len(milestones); got != 3 {
		t.Fatalf("default milestone count = %d, want 3", got)
	}
}
