package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"koschei/api/internal/services"
)

// attachCrossSurfaceIntelligence enriches the canonical token investigation
// with retained actor-coordination and public-promotion evidence. It never
// changes the deterministic ARVIS verdict and never turns correlation into an
// identity, wrongdoing, manipulation or criminal-group claim.
func (h *Handler) attachCrossSurfaceIntelligence(ctx context.Context, report map[string]any) {
	if h == nil || report == nil || isActorDossierReport(report) {
		return
	}
	target := strings.TrimSpace(dossierString(report["target"]))
	network := firstNonEmptyString(strings.TrimSpace(dossierString(report["network"])), "solana-mainnet")
	if target == "" {
		return
	}

	actor := dossierMap(report["actor_investigation"])
	creator := strings.TrimSpace(dossierString(actor["wallet"]))
	var dossier services.ActorDefenseDossier
	decodeCrossSurfaceValue(actor["dossier"], &dossier)
	var genome services.ActorCampaignGenome
	decodeCrossSurfaceValue(actor["campaign_genome"], &genome)

	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	fundingMemory := services.NewPersistentFundingClusterUnavailableReport(
		creator,
		network,
		"not_investigated",
		"A creator/deployer wallet was not available for persistent funding-cluster correlation.",
	)
	if creator != "" {
		loaded, err := services.LoadPersistentFundingClusterHistory(ctx, db, creator, network, 8, 25)
		if err != nil {
			fundingMemory = services.NewPersistentFundingClusterUnavailableReport(
				creator,
				network,
				"query_failed",
				"Persistent funding-cluster memory could not be read; no coordination claim was manufactured.",
			)
		} else {
			fundingMemory = loaded
		}
	}
	coordination := services.BuildActorCoordinationIntelligence(dossier, fundingMemory, genome)
	report["actor_coordination_intelligence"] = coordination
	if actor != nil {
		actor["funding_cluster_memory"] = fundingMemory
		actor["coordination_intelligence"] = coordination
		report["actor_investigation"] = actor
	}

	promotion, promotionErr := services.LoadPublicPromotionIntelligence(ctx, db, network, target, 100, 500)
	if promotionErr != nil && promotion.Status == "" {
		promotion = services.NewPublicPromotionIntelligenceUnavailable(
			network,
			target,
			"source_unavailable",
			"Public promotion intelligence could not be read; no hype/coordination claim was manufactured.",
		)
	}
	report["public_promotion_intelligence"] = promotion

	policy := dossierMap(report["evidence_policy"])
	if policy == nil {
		policy = map[string]any{}
	}
	policy["coordination_intelligence_can_change_grade"] = false
	policy["public_promotion_intelligence_can_change_grade"] = false
	policy["public_promotion_overlap_is_not_operator_identity"] = true
	policy["public_promotion_overlap_is_not_manipulation_verdict"] = true
	policy["coordination_pattern_is_not_criminal_group_claim"] = true
	policy["private_group_collection_without_authorization"] = false
	report["evidence_policy"] = policy
}

func decodeCrossSurfaceValue(value any, out any) {
	if value == nil || out == nil {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = json.Unmarshal(encoded, out)
}
