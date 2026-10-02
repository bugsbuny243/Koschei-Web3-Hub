package services

import "testing"

func TestGlobalCampaignTransitionAllowsEvidenceBoundLifecycle(t *testing.T) {
	current := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-lifecycle",
		State:           GlobalCampaignEmerging,
		ObservationRefs: []string{"obs-1"},
	})
	transition := GlobalCampaignTransition{
		From:         GlobalCampaignEmerging,
		To:           GlobalCampaignActive,
		ReasonCodes:  []string{"VERIFIED_SEQUENCE", " VERIFIED_SEQUENCE "},
		EvidenceRefs: []string{"rel-1", " rel-1 "},
	}
	next, err := ApplyGlobalCampaignTransition(current, transition)
	if err != nil {
		t.Fatalf("apply transition: %v", err)
	}
	if next.Revision != current.Revision+1 {
		t.Fatalf("expected revision %d, got %d", current.Revision+1, next.Revision)
	}
	if next.State != GlobalCampaignActive {
		t.Fatalf("expected active state, got %q", next.State)
	}
	if len(next.TransitionReasonCodes) != 1 || next.TransitionReasonCodes[0] != "VERIFIED_SEQUENCE" {
		t.Fatalf("reason codes were not canonicalized: %#v", next.TransitionReasonCodes)
	}
	if len(next.TransitionEvidenceRefs) != 1 || next.TransitionEvidenceRefs[0] != "rel-1" {
		t.Fatalf("transition evidence refs were not canonicalized: %#v", next.TransitionEvidenceRefs)
	}
	if next.EvidenceHashSHA256 == current.EvidenceHashSHA256 {
		t.Fatal("state transition revision must change campaign evidence hash")
	}
	if next.CampaignRef != current.CampaignRef {
		t.Fatal("state transition must preserve stable campaign reference")
	}
}

func TestGlobalCampaignTransitionRejectsInvalidJump(t *testing.T) {
	transition := GlobalCampaignTransition{
		From:         GlobalCampaignEmerging,
		To:           GlobalCampaignEscalating,
		ReasonCodes:  []string{"OBSERVED_SPIKE"},
		EvidenceRefs: []string{"obs-2"},
	}
	if err := ValidateGlobalCampaignTransition(transition); err == nil {
		t.Fatal("expected emerging -> escalating to require active state first")
	}
}

func TestGlobalCampaignTransitionRequiresEvidence(t *testing.T) {
	transition := GlobalCampaignTransition{
		From:        GlobalCampaignActive,
		To:          GlobalCampaignDormant,
		ReasonCodes: []string{"NO_MATERIAL_ACTIVITY"},
	}
	if err := ValidateGlobalCampaignTransition(transition); err == nil {
		t.Fatal("expected transition without evidence reference to fail")
	}
}

func TestGlobalCampaignClosedCanOnlyReopen(t *testing.T) {
	if !globalCampaignTransitionAllowed(GlobalCampaignClosed, GlobalCampaignReopened) {
		t.Fatal("closed campaign should permit evidence-bound reopen transition")
	}
	if globalCampaignTransitionAllowed(GlobalCampaignClosed, GlobalCampaignActive) {
		t.Fatal("closed campaign must not jump directly to active")
	}
}
