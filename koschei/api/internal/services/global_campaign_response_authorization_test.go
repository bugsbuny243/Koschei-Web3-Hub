package services

import (
	"errors"
	"testing"
	"time"
)

func responseAuthorizationFixture(t *testing.T) (GlobalCampaign, FabricCampaignEvidenceContract, GlobalCampaignResponseProposal, time.Time) {
	t.Helper()
	base := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	campaign := NewGlobalCampaign(GlobalCampaign{
		CampaignRef:         "KCAM1-RESPONSE",
		Revision:            7,
		State:               GlobalCampaignEscalating,
		FirstObservedAt:     base.Add(-2 * time.Hour),
		LastObservedAt:      base.Add(-15 * time.Minute),
		Networks:            []string{"solana", "ethereum"},
		Subjects:            []string{"subject:wallet:a"},
		ObservationRefs:     []string{"obs:1"},
		VerifiedAnchorCount: 1,
		RulesetVersion:      "campaign-rules.v1",
	})
	fabric, err := BuildFabricCampaignEvidenceContract(
		campaign,
		GlobalCampaignTemporalCorrelationReport{},
		GlobalCampaignRadarProjection{},
		GlobalCampaignThreatFamilyReport{},
		nil,
	)
	if err != nil {
		t.Fatalf("build Fabric contract: %v", err)
	}

	proposal, err := NewGlobalCampaignResponseProposal(campaign, fabric, GlobalCampaignResponseProposal{
		PolicyRef:                 "policy:defensive-response:v1",
		PolicyHashSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Network:                   "solana",
		Target:                    "subject:wallet:a",
		ActionKind:                "restrict",
		ActionSHA256:              "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RequestedAt:               base.Add(-5 * time.Minute),
		ExpiresAt:                 base.Add(25 * time.Minute),
		DefensiveOnly:             true,
		BoundedScope:              true,
		AuthorityExpansionAllowed: false,
		AssetTransferAllowed:      false,
	})
	if err != nil {
		t.Fatalf("build response proposal: %v", err)
	}
	return campaign, fabric, proposal, base
}

func TestGlobalCampaignResponseAuthorizationExactOperatorPermit(t *testing.T) {
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	auth, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
		AuthorizerKind:           GlobalCampaignResponseAuthorizerOperator,
		AuthorizerRef:            "operator:oncall-1",
		AuthorizationEvidenceRef: "approval:ticket-42",
		AuthorizedAt:             now.Add(-time.Minute),
		ExpiresAt:                now.Add(10 * time.Minute),
		ExplicitAuthorization:    true,
	}, now)
	if err != nil {
		t.Fatalf("authorize response: %v", err)
	}
	if auth.AuthorizationHashSHA256 == "" {
		t.Fatal("expected immutable authorization hash")
	}
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, auth, now); err != nil {
		t.Fatalf("validate authorization: %v", err)
	}

	again, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
		AuthorizerKind:           GlobalCampaignResponseAuthorizerOperator,
		AuthorizerRef:            "operator:oncall-1",
		AuthorizationEvidenceRef: "approval:ticket-42",
		AuthorizedAt:             now.Add(-time.Minute),
		ExpiresAt:                now.Add(10 * time.Minute),
		ExplicitAuthorization:    true,
	}, now)
	if err != nil {
		t.Fatalf("rebuild authorization: %v", err)
	}
	if auth.AuthorizationHashSHA256 != again.AuthorizationHashSHA256 {
		t.Fatalf("authorization hash is not deterministic: %q != %q", auth.AuthorizationHashSHA256, again.AuthorizationHashSHA256)
	}
}

func TestGlobalCampaignResponseAuthorizationRejectsSentinelOrFabricAuthority(t *testing.T) {
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	for _, kind := range []string{"sentinel", "fabric", "model"} {
		t.Run(kind, func(t *testing.T) {
			_, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
				AuthorizerKind:           kind,
				AuthorizerRef:            kind + ":candidate",
				AuthorizationEvidenceRef: "opinion:1",
				AuthorizedAt:             now.Add(-time.Minute),
				ExpiresAt:                now.Add(10 * time.Minute),
				ExplicitAuthorization:    true,
			}, now)
			if !errors.Is(err, ErrGlobalCampaignResponseUnauthorized) {
				t.Fatalf("expected unauthorized error, got %v", err)
			}
		})
	}
}

func TestGlobalCampaignResponseAuthorizationRejectsCampaignRevisionMismatch(t *testing.T) {
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	changed := campaign
	changed.Revision++
	changed = NewGlobalCampaign(changed)

	_, err := NewGlobalCampaignResponseAuthorization(changed, fabric, proposal, GlobalCampaignResponseAuthorization{
		AuthorizerKind:           GlobalCampaignResponseAuthorizerOperator,
		AuthorizerRef:            "operator:oncall-1",
		AuthorizationEvidenceRef: "approval:ticket-42",
		AuthorizedAt:             now.Add(-time.Minute),
		ExpiresAt:                now.Add(10 * time.Minute),
		ExplicitAuthorization:    true,
	}, now)
	if !errors.Is(err, ErrGlobalCampaignResponseMismatch) {
		t.Fatalf("expected campaign/fabric mismatch, got %v", err)
	}
}

func TestGlobalCampaignResponseProposalRejectsUnsafeScope(t *testing.T) {
	campaign, fabric, _, now := responseAuthorizationFixture(t)
	cases := []struct {
		name   string
		mutate func(*GlobalCampaignResponseProposal)
	}{
		{name: "not defensive", mutate: func(p *GlobalCampaignResponseProposal) { p.DefensiveOnly = false }},
		{name: "unbounded", mutate: func(p *GlobalCampaignResponseProposal) { p.BoundedScope = false }},
		{name: "authority expansion", mutate: func(p *GlobalCampaignResponseProposal) { p.AuthorityExpansionAllowed = true }},
		{name: "asset transfer", mutate: func(p *GlobalCampaignResponseProposal) { p.AssetTransferAllowed = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := GlobalCampaignResponseProposal{
				PolicyRef:                 "policy:defensive-response:v1",
				PolicyHashSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Network:                   "solana",
				Target:                    "subject:wallet:a",
				ActionKind:                "restrict",
				ActionSHA256:              "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				RequestedAt:               now.Add(-time.Minute),
				ExpiresAt:                 now.Add(10 * time.Minute),
				DefensiveOnly:             true,
				BoundedScope:              true,
				AuthorityExpansionAllowed: false,
				AssetTransferAllowed:      false,
			}
			tc.mutate(&input)
			if _, err := NewGlobalCampaignResponseProposal(campaign, fabric, input); !errors.Is(err, ErrGlobalCampaignResponseInvalid) {
				t.Fatalf("expected invalid proposal, got %v", err)
			}
		})
	}
}

func TestGlobalCampaignResponseAuthorizationRejectsExpiryAndTamper(t *testing.T) {
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	auth, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
		AuthorizerKind:           GlobalCampaignResponseAuthorizerOperator,
		AuthorizerRef:            "operator:oncall-1",
		AuthorizationEvidenceRef: "approval:ticket-42",
		AuthorizedAt:             now.Add(-time.Minute),
		ExpiresAt:                now.Add(5 * time.Minute),
		ExplicitAuthorization:    true,
	}, now)
	if err != nil {
		t.Fatalf("authorize response: %v", err)
	}
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, auth, now.Add(6*time.Minute)); !errors.Is(err, ErrGlobalCampaignResponseExpired) {
		t.Fatalf("expected expired authorization, got %v", err)
	}

	auth.ActionSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := ValidateGlobalCampaignResponseAuthorization(campaign, fabric, proposal, auth, now); err == nil {
		t.Fatal("expected tampered authorization to fail closed")
	}
}
