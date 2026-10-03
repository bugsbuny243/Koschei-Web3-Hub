package services

import (
	"testing"
	"time"
)

func TestGlobalCampaignCanonicalDeterminism(t *testing.T) {
	observed := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	a := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-example",
		Revision:        1,
		State:           GlobalCampaignActive,
		FirstObservedAt: observed,
		LastObservedAt:  observed.Add(5 * time.Minute),
		Networks:        []string{"solana-mainnet", "ethereum-mainnet", "solana-mainnet"},
		Actors:          []string{"actor-b", "actor-a", "actor-a"},
		Assets:          []string{"asset-x"},
		ObservationRefs: []string{"obs-2", "obs-1", "obs-1"},
		RelationRefs:    []string{"rel-1"},
		BridgeLinkRefs:  []string{"bridge-1"},
		RulesetVersion:  "campaign-rules-v1",
	})
	b := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-example",
		Revision:        1,
		State:           GlobalCampaignActive,
		FirstObservedAt: observed,
		LastObservedAt:  observed.Add(5 * time.Minute),
		Networks:        []string{"ethereum-mainnet", "solana-mainnet"},
		Actors:          []string{"actor-a", "actor-b"},
		Assets:          []string{"asset-x"},
		ObservationRefs: []string{"obs-1", "obs-2"},
		RelationRefs:    []string{"rel-1"},
		BridgeLinkRefs:  []string{"bridge-1"},
		RulesetVersion:  "campaign-rules-v1",
	})

	if a.EvidenceHashSHA256 == "" {
		t.Fatal("expected campaign evidence hash")
	}
	if a.EvidenceHashSHA256 != b.EvidenceHashSHA256 {
		t.Fatalf("canonical campaign hash changed with input ordering: %q != %q", a.EvidenceHashSHA256, b.EvidenceHashSHA256)
	}
	if len(a.Networks) != 2 || a.Networks[0] != "ethereum-mainnet" || a.Networks[1] != "solana-mainnet" {
		t.Fatalf("networks were not normalized: %#v", a.Networks)
	}
	if len(a.Actors) != 2 || a.Actors[0] != "actor-a" || a.Actors[1] != "actor-b" {
		t.Fatalf("actors were not normalized: %#v", a.Actors)
	}
}

func TestGlobalCampaignAuthorityBoundaryFailsClosed(t *testing.T) {
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:            "KCAM1-boundary",
		State:                  GlobalCampaignEscalating,
		VerdictAuthority:       true,
		GradeAuthority:         true,
		ContainmentAuthority:   true,
		SameOperatorClaim:      true,
		RealWorldIdentityClaim: true,
		WrongdoingClaim:        true,
		VerifiedAnchorCount:    -1,
		ObservedAnchorCount:    -1,
		TransitionReasonCodes:  []string{" VERIFIED_BRIDGE_LINK ", "VERIFIED_BRIDGE_LINK"},
		TransitionEvidenceRefs: []string{" rel-1 ", "rel-1"},
	})

	if campaign.VerdictAuthority || campaign.GradeAuthority || campaign.ContainmentAuthority {
		t.Fatal("global campaign contract must never gain decision or containment authority")
	}
	if campaign.SameOperatorClaim || campaign.RealWorldIdentityClaim || campaign.WrongdoingClaim {
		t.Fatal("global campaign contract must not make identity, common-control, or wrongdoing claims")
	}
	if campaign.VerifiedAnchorCount != 0 || campaign.ObservedAnchorCount != 0 {
		t.Fatalf("negative anchor counts must normalize to zero: verified=%d observed=%d", campaign.VerifiedAnchorCount, campaign.ObservedAnchorCount)
	}
	if campaign.Revision != 1 {
		t.Fatalf("zero revision must normalize to one, got %d", campaign.Revision)
	}
	if len(campaign.TransitionReasonCodes) != 1 || campaign.TransitionReasonCodes[0] != "VERIFIED_BRIDGE_LINK" {
		t.Fatalf("transition reason codes were not canonicalized: %#v", campaign.TransitionReasonCodes)
	}
}

func TestGlobalCampaignEvidenceRevisionChangesHashWithoutChangingRef(t *testing.T) {
	base := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-stable",
		Revision:        1,
		State:           GlobalCampaignEmerging,
		ObservationRefs: []string{"obs-1"},
	})
	next := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     base.CampaignRef,
		Revision:        2,
		State:           GlobalCampaignActive,
		ObservationRefs: []string{"obs-1", "obs-2"},
	})

	if base.CampaignRef != next.CampaignRef {
		t.Fatalf("campaign reference must remain stable across revisions: %q != %q", base.CampaignRef, next.CampaignRef)
	}
	if base.EvidenceHashSHA256 == next.EvidenceHashSHA256 {
		t.Fatal("material evidence revision must change evidence hash")
	}
}

func TestGlobalCampaignNormalizesObservationWindow(t *testing.T) {
	late := time.Date(2026, 10, 2, 1, 0, 0, 0, time.FixedZone("test", 3*60*60))
	early := late.Add(-10 * time.Minute)
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:     "KCAM1-time",
		FirstObservedAt: late,
		LastObservedAt:  early,
		State:           "not-a-state",
	})

	if campaign.State != GlobalCampaignEmerging {
		t.Fatalf("invalid campaign state must fail to emerging, got %q", campaign.State)
	}
	if campaign.LastObservedAt.Before(campaign.FirstObservedAt) {
		t.Fatalf("campaign observation window is inverted: first=%s last=%s", campaign.FirstObservedAt, campaign.LastObservedAt)
	}
	if campaign.FirstObservedAt.Location() != time.UTC || campaign.LastObservedAt.Location() != time.UTC {
		t.Fatal("campaign observation timestamps must be canonical UTC")
	}
}
