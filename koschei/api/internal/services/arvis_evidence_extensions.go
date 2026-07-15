package services

import (
	"fmt"
	"strings"
	"time"
)

// ApplyCreatorAndLiquidityEvidenceToAnalysis replaces the two base placeholders
// after the handler has collected source metadata and market liquidity. It does
// not recalculate holder evidence and cannot issue a grade.
func ApplyCreatorAndLiquidityEvidenceToAnalysis(analysis ArvisAnalysis, req SecurityRadarRequest, creator string, market TokenMarketSnapshot, launch LaunchForensicsAnalysis) ArvisAnalysis {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	creator = strings.TrimSpace(creator)
	creatorArm := creatorLinkEvidenceArm(req, creator, launch, generatedAt)
	liquidityArm := liquidityMovementEvidenceArm(req, market, generatedAt)

	arms := ArvisArmsFromBundle(analysis.Bundle)
	if len(arms) == 0 {
		arms = append([]SecurityRadarVerdict{}, analysis.Arms...)
	}
	updated := make([]SecurityRadarVerdict, 0, len(arms))
	creatorFound := false
	liquidityFound := false
	for _, arm := range arms {
		switch arm.ModuleID {
		case ModuleCreatorLinkAnalysis:
			updated = append(updated, creatorArm)
			creatorFound = true
		case ModuleLiquidityMovement:
			updated = append(updated, liquidityArm)
			liquidityFound = true
		default:
			updated = append(updated, arm)
		}
	}
	if !creatorFound {
		updated = append(updated, creatorArm)
	}
	if !liquidityFound {
		updated = append(updated, liquidityArm)
	}
	analysis.Arms = updated
	analysis.Final = arvisCompatibilityFinal()
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = updated
	analysis.Bundle.Metadata["creator_link_analysis"] = creatorArm
	analysis.Bundle.Metadata["liquidity_movement"] = liquidityArm
	analysis.Bundle.Metadata["verified_arm_count"] = verifiedArvisEvidenceCount(updated)
	analysis.Bundle.Metadata["runtime_arm_count"] = verifiedArvisEvidenceCount(updated)
	analysis.Bundle.Metadata["final_verdict_source"] = "EvaluateUnifiedRadarVerdict"
	return analysis
}

func creatorLinkEvidenceArm(req SecurityRadarRequest, creator string, launch LaunchForensicsAnalysis, generatedAt string) SecurityRadarVerdict {
	if creator == "" {
		return unavailableArm("Creator Link Analysis", ModuleCreatorLinkAnalysis, req, generatedAt, "Creator/deployer wallet was not present in the source or parsed launch context.")
	}
	signals := map[string]any{
		"module_id": ModuleCreatorLinkAnalysis,
		"real_onchain_evidence": true,
		"evidence_status": "observed",
		"creator_wallet": creator,
		"observed_role": "creator_deployer",
		"creator_linked_launch_actor_count": launch.CreatorLinkedCount,
		"launch_data_source": launch.DataSource,
		"identity_or_wrongdoing_claim": false,
		"numeric_score_disabled": true,
		"grade_effect": "none_at_arm_layer",
	}
	evidence := []string{
		fmt.Sprintf("Creator/deployer wallet observed for mint %s: %s.", req.Target, creator),
		"This relation is an on-chain/source observation and is not a real-world identity or wrongdoing claim.",
	}
	if launch.CreatorLinkedCount > 0 {
		evidence = append(evidence, fmt.Sprintf("Launch analysis observed %d recipient/holder profile(s) with creator-linked funding evidence.", launch.CreatorLinkedCount))
	}
	arm := evidenceArm("Creator Link Analysis", ModuleCreatorLinkAnalysis, req, 0, signals, evidence, generatedAt)
	arm.Verdict = "Creator/deployer relation observed; cross-token reuse is owned by Repeat Actor Scan and final interpretation belongs to the unified rules engine."
	arm.Recommendation = "Inspect persistent created-token history and direct creator-to-holder transaction evidence."
	return arm
}

func liquidityMovementEvidenceArm(req SecurityRadarRequest, market TokenMarketSnapshot, generatedAt string) SecurityRadarVerdict {
	if !market.Available || market.LiquidityUSD <= 0 {
		v := unavailableArm("Liquidity Movement", ModuleLiquidityMovement, req, generatedAt, "A positive pool-liquidity snapshot or parsed LP transaction is required; missing liquidity is not a low-risk signal.")
		v.Signals["market_snapshot"] = market
		for _, limitation := range market.Limitations {
			v.Evidence = append(v.Evidence, "Limitation: "+limitation)
		}
		return v
	}
	observedAt := market.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	signals := map[string]any{
		"module_id": ModuleLiquidityMovement,
		"real_offchain_evidence": true,
		"evidence_status": "observed_market_snapshot",
		"provider": market.Provider,
		"liquidity_usd": market.LiquidityUSD,
		"best_pair_address": market.BestPairAddress,
		"best_pair_dex": market.BestPairDEX,
		"best_pair_liquidity_usd": market.BestPairLiquidityUSD,
		"best_pair_volume_24h_usd": market.BestPairVolume24hUSD,
		"pair_count": market.PairCount,
		"observed_at": observedAt.UTC().Format(time.RFC3339),
		"valuation_scope": market.ValuationScope,
		"lp_add_remove_transaction_verified": false,
		"numeric_score_disabled": true,
		"grade_effect": "none_at_arm_layer",
	}
	evidence := []string{
		fmt.Sprintf("Market provider %s reported $%.2f liquidity across %d Solana pair(s).", market.Provider, market.LiquidityUSD, market.PairCount),
		fmt.Sprintf("Most liquid pair %s on %s reported $%.2f liquidity.", firstRadarValue(market.BestPairAddress, "unknown"), firstRadarValue(market.BestPairDEX, "unknown"), market.BestPairLiquidityUSD),
		"This snapshot describes pool depth; it does not claim who added or removed liquidity without parsed LP transaction evidence.",
	}
	for _, limitation := range market.Limitations {
		evidence = append(evidence, "Limitation: "+limitation)
	}
	arm := evidenceArm("Liquidity Movement", ModuleLiquidityMovement, req, 0, signals, evidence, generatedAt)
	arm.Verdict = "Liquidity depth was observed. LP actor attribution remains unverified until transaction-backed add/remove evidence exists."
	arm.Recommendation = "Use liquidity depth as input to URD-C001/URD-C002 and parsed LP transactions for creator-removal hard triggers."
	return arm
}
