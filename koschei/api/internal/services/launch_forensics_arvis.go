package services

import (
	"fmt"
	"time"
)

// ApplyLaunchForensicsToAnalysis replaces the Launch Distribution placeholder
// with mint-specific ATA/ledger evidence. It never rebuilds a final score: the
// unified deterministic rules engine remains the only verdict source.
func ApplyLaunchForensicsToAnalysis(analysis ArvisAnalysis, req SecurityRadarRequest, forensics LaunchForensicsAnalysis) ArvisAnalysis {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	arms := ArvisArmsFromBundle(analysis.Bundle)
	if len(arms) == 0 {
		arms = append([]SecurityRadarVerdict{}, analysis.Arms...)
	}
	updated := make([]SecurityRadarVerdict, 0, len(arms))
	found := false
	for _, arm := range arms {
		if arm.ModuleID != ModuleLaunchDistribution {
			updated = append(updated, arm)
			continue
		}
		found = true
		updated = append(updated, launchDistributionArm(req, forensics, generatedAt))
	}
	if !found {
		updated = append(updated, launchDistributionArm(req, forensics, generatedAt))
	}
	analysis.Arms = updated
	analysis.Final = arvisCompatibilityFinal()
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = updated
	analysis.Bundle.Metadata["architecture_arm_count"] = 14
	analysis.Bundle.Metadata["evidence_arm_count"] = 14
	analysis.Bundle.Metadata["verified_arm_count"] = verifiedArvisEvidenceCount(updated)
	analysis.Bundle.Metadata["runtime_arm_count"] = verifiedArvisEvidenceCount(updated)
	analysis.Bundle.Metadata["launch_distribution"] = forensics
	analysis.Bundle.Metadata["final_verdict_source"] = "EvaluateUnifiedRadarVerdict"
	analysis.Bundle.CustomerRecommendation = "evaluate_unified_rules"
	return analysis
}

func launchDistributionArm(req SecurityRadarRequest, forensics LaunchForensicsAnalysis, generatedAt string) SecurityRadarVerdict {
	if !forensics.Available {
		v := unavailableArm("Launch Distribution", ModuleLaunchDistribution, req, generatedAt, "Live ledger or mint-specific ATA recipient history is required; missing history is not a safety signal.")
		v.Signals["launch_forensics"] = forensics
		for _, limitation := range forensics.Limitations {
			v.Evidence = append(v.Evidence, "Limitation: "+limitation)
		}
		return v
	}
	signals := map[string]any{
		"module_id": ModuleLaunchDistribution,
		"real_onchain_evidence": true,
		"verified_evidence": true,
		"evidence_status": "verified_launch_distribution",
		"data_source": forensics.DataSource,
		"owners_requested": forensics.OwnersRequested,
		"owners_with_trade_history": forensics.OwnersWithTradeHistory,
		"ledger_trade_count": forensics.LedgerTradeCount,
		"launch_slot": forensics.LaunchSlot,
		"launch_time": forensics.LaunchTime,
		"profiles": forensics.Profiles,
		"timeline": forensics.Timeline,
		"sniper_count": forensics.SniperCount,
		"rhythm_bot_count": forensics.RhythmBotCount,
		"creator_linked_count": forensics.CreatorLinkedCount,
		"rpc_calls_used": forensics.RPCCallsUsed,
		"funding_rpc_calls_used": forensics.FundingRPCCallsUsed,
		"ata_only_policy": true,
		"recipient_wide_wallet_history_forbidden": true,
		"numeric_score_disabled": true,
		"grade_effect": "none_at_arm_layer",
	}
	evidence := append([]string{}, forensics.Findings...)
	if len(evidence) == 0 && forensics.Summary != "" {
		evidence = append(evidence, forensics.Summary)
	}
	for _, limitation := range forensics.Limitations {
		evidence = append(evidence, "Limitation: "+limitation)
	}
	arm := evidenceArm("Launch Distribution", ModuleLaunchDistribution, req, 0, signals, evidence, generatedAt)
	arm.Verdict = fmt.Sprintf("Launch Distribution observed mint-specific history for %d owner wallets; this arm reports evidence and does not issue a grade.", forensics.OwnersWithTradeHistory)
	arm.Recommendation = "Compare initial recipients with current top holders and persistent repeat-actor evidence."
	return arm
}
