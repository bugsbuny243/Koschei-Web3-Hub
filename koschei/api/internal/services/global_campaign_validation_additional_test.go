package services

import "testing"

func TestValidateGlobalCampaignRequiresIdentityAndEvidence(t *testing.T) {
	missingRef := NewGlobalCampaign(GlobalCampaign{ObservationRefs: []string{"obs-1"}})
	if err := ValidateGlobalCampaign(missingRef); err == nil {
		t.Fatal("expected empty campaign reference to fail validation")
	}

	missingEvidence := NewGlobalCampaign(GlobalCampaign{CampaignRef: "KCAM1-no-evidence"})
	if err := ValidateGlobalCampaign(missingEvidence); err == nil {
		t.Fatal("expected campaign without evidence anchors or refs to fail validation")
	}

	valid := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-valid",
		ObservationRefs: []string{"obs-1"},
	})
	if err := ValidateGlobalCampaign(valid); err != nil {
		t.Fatalf("expected canonical campaign to validate: %v", err)
	}
}

func TestValidateGlobalCampaignRejectsTamperedHash(t *testing.T) {
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-hash",
		ObservationRefs: []string{"obs-1"},
	})
	campaign.EvidenceHashSHA256 = "sha256:tampered"
	if err := ValidateGlobalCampaign(campaign); err == nil {
		t.Fatal("expected tampered campaign evidence hash to fail validation")
	}
}
