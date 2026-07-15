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
	replaceArvisArmPreservingVerifiedSource(arms, buildPumpTransactionArm(req, txEvidence, generatedAt))
	replaceArvisArmPreservingVerifiedSource(arms, buildRaydiumTransactionArm(req, txEvidence, generatedAt))
	replaceArvisArm(arms, buildTransactionMEVArm(req, txEvidence, generatedAt))
	replaceArvisArm(arms, buildLiquidityMovementTransactionArm(req, txEvidence, generatedAt))
	replaceArvisArm(arms, buildCreatorLinkTransactionArm(req, txEvidence, generatedAt))
	replaceFundingClusterArmPreservingHolderEvidence(arms, buildFundingClusterTransactionArm(req, txEvidence, generatedAt))

	verified := verifiedArvisEvidenceCount(arms)
	bundle.Metadata["arvis_arms"] = arms
	bundle.Metadata["verified_arm_count"] = verified
	bundle.Metadata["runtime_arm_count"] = verified
	bundle.Metadata["transaction_evidence_available"] = true
	bundle.Metadata["transaction_signature"] = txEvidence.Signature
	bundle.Metadata["transaction_program_count"] = len(txEvidence.ProgramIDs)
	bundle.Metadata["transaction_signer_count"] = len(txEvidence.Signers)
	bundle.Metadata["pump_program_related"] = txEvidence.PumpRelated
	bundle.Metadata["raydium_program_related"] = txEvidence.RaydiumRelated
	bundle.Metadata["final_verdict_source"] = "EvaluateUnifiedRadarVerdict"
	bundle.CustomerRecommendation = "evaluate_unified_rules"
	bundle.CustomerSummary = fmt.Sprintf("ARVIS collected parsed transaction evidence in %d of 14 single-responsibility arms; no arm issued a grade.", verified)
	return bundle
}

func replaceFundingClusterArmPreservingHolderEvidence(arms []SecurityRadarVerdict, replacement SecurityRadarVerdict) {
	for i := range arms {
		if arms[i].ModuleID != ModuleFundingClusterDetector {
			continue
		}
		_, hasHolderCluster := arms[i].Signals["holder_cluster_analysis"]
		if hasHolderCluster {
			if arms[i].Signed && SecurityRadarVerdictHasVerifiedEvidence(arms[i]) {
				return
			}
			if !replacement.Signed {
				return
			}
		}
		if !replacement.Signed && arms[i].Signed && SecurityRadarVerdictHasVerifiedEvidence(arms[i]) {
			return
		}
		arms[i] = replacement
		return
	}
}

func replaceArvisArmPreservingVerifiedSource(arms []SecurityRadarVerdict, replacement SecurityRadarVerdict) {
	for i := range arms {
		if arms[i].ModuleID != replacement.ModuleID {
			continue
		}
		if !replacement.Signed && arms[i].Signed && SecurityRadarVerdictHasVerifiedEvidence(arms[i]) {
			return
		}
		arms[i] = replacement
		return
	}
}
