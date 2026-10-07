package handlers

import (
	"context"
	"testing"

	"koschei/api/internal/services"
)

func TestAttachCrossSurfaceIntelligenceKeepsCorrelationNonAuthoritative(t *testing.T) {
	dossier := services.ActorDefenseDossier{
		Wallet:  "Creator111",
		Network: "solana-mainnet",
		Track: services.ActorDefenseTrack{
			Network:           "solana-mainnet",
			TargetKind:        "wallet",
			TargetID:          "Creator111",
			CreatedTokenCount: 2,
			Dossier:           map[string]any{"creator_reuse_evidence_status": "verified"},
		},
	}
	report := map[string]any{
		"schema_version": unifiedInvestigationSchemaVersion,
		"target":         "Mint111",
		"network":        "solana-mainnet",
		"actor_investigation": map[string]any{
			"wallet":          "Creator111",
			"dossier":         dossier,
			"campaign_genome": services.ActorCampaignGenome{},
		},
		"evidence_policy": map[string]any{},
	}

	(&Handler{}).attachCrossSurfaceIntelligence(context.Background(), report)

	coordination, ok := report["actor_coordination_intelligence"].(services.ActorCoordinationIntelligence)
	if !ok {
		t.Fatalf("coordination type=%T", report["actor_coordination_intelligence"])
	}
	if coordination.Status != "coordination_patterns_observed" || coordination.SignalCount < 1 {
		t.Fatalf("coordination=%#v", coordination)
	}
	if coordination.SameOperatorClaim || coordination.RealWorldIdentityClaim || coordination.CriminalGroupClaim || coordination.MarketManipulationClaim || coordination.VerdictAuthority {
		t.Fatalf("coordination became authoritative attribution: %#v", coordination)
	}

	market, ok := report["market_manipulation_intelligence"].(services.MarketManipulationIntelligence)
	if !ok {
		t.Fatalf("market intelligence type=%T", report["market_manipulation_intelligence"])
	}
	if market.Status != "trade_ledger_unavailable" || market.WashTradingProven || market.ManipulationClaim || market.SameOperatorClaim || market.VerdictAuthority {
		t.Fatalf("market intelligence became an unsupported claim: %#v", market)
	}

	promotion, ok := report["public_promotion_intelligence"].(services.PublicPromotionIntelligence)
	if !ok {
		t.Fatalf("promotion type=%T", report["public_promotion_intelligence"])
	}
	if promotion.Status != "source_unavailable" || promotion.VerdictAuthority || promotion.GradeAuthority || promotion.SameOperatorClaim || promotion.WrongdoingClaim || promotion.MarketManipulationClaim {
		t.Fatalf("promotion=%#v", promotion)
	}

	actor := dossierMap(report["actor_investigation"])
	if _, ok := actor["funding_cluster_memory"]; !ok {
		t.Fatal("funding_cluster_memory was not projected into actor investigation")
	}
	if _, ok := actor["coordination_intelligence"]; !ok {
		t.Fatal("coordination_intelligence was not projected into actor investigation")
	}

	policy := dossierMap(report["evidence_policy"])
	if policy["coordination_intelligence_can_change_grade"] != false || policy["market_manipulation_intelligence_can_change_grade"] != false || policy["public_promotion_intelligence_can_change_grade"] != false {
		t.Fatalf("policy=%#v", policy)
	}
}

func TestUnifiedInvestigationProjectionKeepsCrossSurfaceFields(t *testing.T) {
	report := map[string]any{
		"schema_version":                         unifiedInvestigationSchemaVersion,
		"target":                                 "Mint111",
		"network":                                "solana-mainnet",
		"actor_coordination_intelligence":        map[string]any{"status": "coordination_patterns_observed"},
		"market_manipulation_intelligence":       map[string]any{"status": "round_trip_churn_candidates_observed"},
		"public_promotion_intelligence":          map[string]any{"status": "cross_asset_public_promotion_overlap_observed"},
		"creator_intelligence":                   map[string]any{"available": true},
		"creator_distribution":                   map[string]any{"available": true},
		"internal_request_only_should_be_hidden": true,
	}
	projected := unifiedInvestigationTechnicalProjection(report)
	for _, key := range []string{"actor_coordination_intelligence", "market_manipulation_intelligence", "public_promotion_intelligence", "creator_intelligence", "creator_distribution"} {
		if _, ok := projected[key]; !ok {
			t.Fatalf("%s missing from projection: %#v", key, projected)
		}
	}
	if _, ok := projected["internal_request_only_should_be_hidden"]; ok {
		t.Fatal("projection leaked an unapproved field")
	}
}
