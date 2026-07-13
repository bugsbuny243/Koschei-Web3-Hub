package services

import (
	"fmt"
	"time"
)

// ApplyLaunchForensicsToAnalysis adds one deterministic evidence arm and
// rebuilds the existing final engine. Thresholds and correlation rules remain
// unchanged; only proven launch timing/funding evidence can raise risk.
func ApplyLaunchForensicsToAnalysis(analysis ArvisAnalysis, req SecurityRadarRequest, forensics LaunchForensicsAnalysis) ArvisAnalysis {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	arms := ArvisArmsFromBundle(analysis.Bundle)
	if len(arms) == 0 {
		arms = append([]SecurityRadarVerdict{}, analysis.Arms...)
	}
	withoutFinal := make([]SecurityRadarVerdict, 0, len(arms)+1)
	for _, arm := range arms {
		if arm.ModuleID != ModuleFinalVerdictEngine {
			withoutFinal = append(withoutFinal, arm)
		}
	}
	launchArm := unavailableArm("Launch Forensics", ModuleLaunchForensics, req, generatedAt, "Live ledger or ATA target-token history is required; missing launch history is not a safety signal.")
	if forensics.Available {
		risk := forensics.StructuralFloor
		signals := map[string]any{
			"real_onchain_evidence": true, "verified_evidence": true,
			"evidence_status":                    "verified_launch_forensics",
			"data_source":                        forensics.DataSource,
			"owners_with_trade_history":          forensics.OwnersWithTradeHistory,
			"sniper_count":                       forensics.SniperCount,
			"rhythm_bot_count":                   forensics.RhythmBotCount,
			"creator_linked_count":               forensics.CreatorLinkedCount,
			"funding_owners_attempted":           forensics.FundingOwnersAttempted,
			"funding_owners_resolved":            forensics.FundingOwnersResolved,
			"launch_forensics_risk_contribution": forensics.RiskContribution,
			"launch_forensics_structural_floor":  forensics.StructuralFloor,
			"absence_is_safety_signal":           false,
		}
		evidence := append([]string{}, forensics.Findings...)
		if len(evidence) == 0 && forensics.Summary != "" {
			evidence = append(evidence, forensics.Summary)
		}
		launchArm = evidenceArm("Launch Forensics", ModuleLaunchForensics, req, risk, signals, evidence, generatedAt)
		launchArm.Verdict = fmt.Sprintf("Launch Forensics verified %d holder trade histories: %d sniper, %d rhythm-bot and %d creator-linked relations.", forensics.OwnersWithTradeHistory, forensics.SniperCount, forensics.RhythmBotCount, forensics.CreatorLinkedCount)
	}
	withoutFinal = append(withoutFinal, launchArm)
	finalArm := buildFinalArm(req, withoutFinal, generatedAt)
	arms = append(withoutFinal, finalArm)
	final := finalVerdictFromArm(finalArm)
	verified := verifiedArvisEvidenceCount(arms)

	bundle := analysis.Bundle
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	bundle.Metadata["arvis_arms"] = arms
	bundle.Metadata["architecture_arm_count"] = 15
	bundle.Metadata["evidence_arm_count"] = 14
	bundle.Metadata["verified_arm_count"] = verified
	bundle.Metadata["runtime_arm_count"] = verified
	bundle.Metadata["launch_forensics"] = forensics
	bundle.Metadata["final_grade"] = final.Grade
	bundle.Metadata["final_risk_index"] = final.RiskIndex
	bundle.Metadata["final_risk_level"] = final.RiskLevel
	bundle.Metadata["final_recommendation"] = final.Recommendation
	bundle.CustomerRecommendation = final.Recommendation
	if final.Signed {
		bundle.CustomerSummary = fmt.Sprintf("ARVIS verified %d of 14 evidence arms, including deterministic Launch Forensics, and produced one evidence-backed verdict.", verified)
	}
	return ArvisAnalysis{Bundle: bundle, Arms: arms, Final: final}
}
