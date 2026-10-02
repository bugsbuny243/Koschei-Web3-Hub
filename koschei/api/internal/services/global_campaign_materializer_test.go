package services

import (
	"testing"
	"time"
)

func TestMaterializeGlobalCampaignIsOrderIndependent(t *testing.T) {
	observed := time.Date(2026, 10, 2, 0, 30, 0, 0, time.UTC)
	a := MaterializeGlobalCampaign(GlobalCampaignMaterializerInput{
		ObservedAt:          observed,
		Networks:            []string{"ethereum-mainnet", "solana-mainnet"},
		Actors:              []string{"actor-b", "actor-a"},
		ObservationRefs:     []string{"obs-2", "obs-1"},
		RelationRefs:        []string{"rel-2", "rel-1"},
		BridgeLinkRefs:      []string{"bridge-1"},
		VerifiedAnchorCount: 2,
		ObservedAnchorCount: 1,
		RulesetVersion:      "campaign-rules-v1",
	})
	b := MaterializeGlobalCampaign(GlobalCampaignMaterializerInput{
		ObservedAt:          observed,
		Networks:            []string{"solana-mainnet", "ethereum-mainnet"},
		Actors:              []string{"actor-a", "actor-b"},
		ObservationRefs:     []string{"obs-1", "obs-2"},
		RelationRefs:        []string{"rel-1", "rel-2"},
		BridgeLinkRefs:      []string{"bridge-1"},
		VerifiedAnchorCount: 2,
		ObservedAnchorCount: 1,
		RulesetVersion:      "campaign-rules-v1",
	})

	if a.CampaignRef == "" || a.CampaignRef != b.CampaignRef {
		t.Fatalf("campaign materialization identity must converge: %q != %q", a.CampaignRef, b.CampaignRef)
	}
	if a.EvidenceHashSHA256 != b.EvidenceHashSHA256 {
		t.Fatalf("campaign materialization hash must converge: %q != %q", a.EvidenceHashSHA256, b.EvidenceHashSHA256)
	}
	if err := ValidateGlobalCampaign(a); err != nil {
		t.Fatalf("materialized campaign failed validation: %v", err)
	}
}

func TestMaterializeGlobalCampaignPreservesExistingIdentity(t *testing.T) {
	first := MaterializeGlobalCampaign(GlobalCampaignMaterializerInput{
		ObservedAt:      time.Date(2026, 10, 2, 0, 30, 0, 0, time.UTC),
		RelationRefs:    []string{"rel-1"},
		ObservationRefs: []string{"obs-1"},
	})
	second := MaterializeGlobalCampaign(GlobalCampaignMaterializerInput{
		ExistingCampaignRef: first.CampaignRef,
		ExistingRevision:    first.Revision + 1,
		ExistingState:       GlobalCampaignActive,
		ObservedAt:          time.Date(2026, 10, 2, 0, 35, 0, 0, time.UTC),
		RelationRefs:        []string{"rel-1", "rel-2"},
		ObservationRefs:     []string{"obs-1", "obs-2"},
	})

	if first.CampaignRef == "" {
		t.Fatal("first materialization did not mint a campaign reference")
	}
	if second.CampaignRef != first.CampaignRef {
		t.Fatalf("existing campaign identity changed across evidence growth: %q != %q", second.CampaignRef, first.CampaignRef)
	}
	if second.Revision != 2 {
		t.Fatalf("expected explicit existing revision 2, got %d", second.Revision)
	}
	if second.EvidenceHashSHA256 == first.EvidenceHashSHA256 {
		t.Fatal("material evidence growth should change evidence hash")
	}
}

func TestMaterializeGlobalCampaignRequiresEvidenceForValidation(t *testing.T) {
	campaign := MaterializeGlobalCampaign(GlobalCampaignMaterializerInput{
		Subjects: []string{"subject-only"},
	})
	if err := ValidateGlobalCampaign(campaign); err == nil {
		t.Fatal("entity-only materialization must not validate without evidence anchors or references")
	}
}
