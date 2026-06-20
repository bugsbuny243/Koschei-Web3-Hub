package services

import (
	"fmt"
	"time"
)

func EnrichArvisBundleWithTransactions(bundle SecurityRadarBundle) SecurityRadarBundle {
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	if attempted, _ := bundle.Metadata["transaction_enrichment_attempted"].(bool); attempted {
		return bundle
	}
	bundle.Metadata["transaction_enrichment_attempted"] = true
	arms := ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		return bundle
	}
	req := SecurityRadarRequest{Target: bundle.Target, Network: bundle.Network, Mode: bundle.WatchMode}
	txEvidence := collectArvisTransactionEvidence(req, arms)
	if !txEvidence.Available {
		bundle.Metadata["transaction_evidence_available"] = false
		bundle.Metadata["transaction_evidence_errors"] = txEvidence.Errors
		return bundle
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)
	replaceArvisArm(arms, buildTransactionMEVArm(req, txEvidence, generatedAt))
	replaceArvisArm(arms, buildLiquidityMovementTransactionArm(req, txEvidence, generatedAt))
	replaceArvisArm(arms, buildCreatorLinkTransactionArm(req, txEvidence, generatedAt))
	replaceArvisArm(arms, buildFundingClusterTransactionArm(req, txEvidence, generatedAt))

	withoutFinal := make([]SecurityRadarVerdict, 0, len(arms)-1)
	for _, arm := range arms {
		if arm.ModuleID != ModuleFinalVerdictEngine {
			withoutFinal = append(withoutFinal, arm)
		}
	}
	finalArm := buildFinalArm(req, withoutFinal, generatedAt)
	replaceArvisArm(arms, finalArm)
	final := finalVerdictFromArm(finalArm)
	verified := verifiedArvisArmCount(arms)

	bundle.Metadata["arvis_arms"] = arms
	bundle.Metadata["verified_arm_count"] = verified
	bundle.Metadata["runtime_arm_count"] = verified
	bundle.Metadata["transaction_evidence_available"] = true
	bundle.Metadata["transaction_signature"] = txEvidence.Signature
	bundle.Metadata["transaction_program_count"] = len(txEvidence.ProgramIDs)
	bundle.Metadata["transaction_signer_count"] = len(txEvidence.Signers)
	bundle.Metadata["final_grade"] = final.Grade
	bundle.Metadata["final_risk_index"] = final.RiskIndex
	bundle.Metadata["final_risk_level"] = final.RiskLevel
	bundle.Metadata["final_recommendation"] = final.Recommendation
	bundle.CustomerRecommendation = final.Recommendation
	if final.Signed {
		bundle.CustomerSummary = fmt.Sprintf("ARVIS verified %d of 13 evidence arms, including parsed transaction evidence, and produced one signed verdict.", verified)
	}
	return bundle
}
