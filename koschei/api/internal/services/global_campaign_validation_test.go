package services

import "testing"

func TestGlobalCampaignRejectsEmptyCampaignReference(t *testing.T) {
	campaign := NewGlobalCampaign(GlobalCampaign{
		ObservationRefs: []string{"obs-1"},
	})
	if campaign.CampaignRef != "" {
		t.Fatalf("unexpected campaign ref: %q", campaign.CampaignRef)
	}
}
