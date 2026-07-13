package handlers

import (
	"context"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// actorSecurityIntelligenceForDetail reuses the fresh holder evidence already
// collected by a full Radar scan and joins it with bounded creator/deployer
// evidence. It returns partial/unknown states instead of inventing identity.
func (h *Handler) actorSecurityIntelligenceForDetail(parent context.Context, target, network string, source map[string]any, roles services.HolderRoleAnalysis, cluster services.HolderClusterAnalysis, market services.TokenMarketSnapshot) map[string]any {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	holder := services.BuildHolderIntelligence(roles, cluster, market, time.Now().UTC())
	creatorWallet := strings.TrimSpace(creatorIntelCleanString(source["creator_wallet"]))
	creator := map[string]any{
		"available":      false,
		"status":         "creator_wallet_not_observed",
		"creator_wallet": creatorWallet,
		"summary":        "Creator/deployer cüzdanı doğrulanamadı; kimlik veya bağlantı iddiası üretilmedi.",
	}
	if creatorWallet != "" {
		creator = h.buildCreatorWalletIntelligence(ctx, target, network, creatorWallet, source)
	}
	actor := buildActorSecurityIntelligence(target, source, creator, holder, cluster)
	actor["generated_at"] = time.Now().UTC().Format(time.RFC3339)
	actor["source"] = "full_radar_fresh_holder_evidence"
	actor["creator_analysis_bounded"] = true
	if ctx.Err() != nil {
		actor["status"] = "partial_timeout"
		actor["limitation"] = "Creator/deployer davranış penceresi süre sınırına ulaştı; holder sahiplik kanıtı korunarak kısmi sonuç döndürüldü."
	}
	return actor
}
