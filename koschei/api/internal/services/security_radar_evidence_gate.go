package services

const SecurityRadarInsufficientEvidenceMessage = "Real data unavailable. Analysis could not be completed."

func SecurityRadarHasLiveEvidence(bundle SecurityRadarBundle) bool {
	for _, verdict := range []SecurityRadarVerdict{bundle.PumpSybilRadar, bundle.RaydiumPoolGuardian} {
		if verdict.Signals == nil {
			continue
		}
		if ok, _ := verdict.Signals["real_onchain_evidence"].(bool); ok {
			return true
		}
	}
	return false
}

func EvidenceBackedSecurityRadarBundle(bundle SecurityRadarBundle) SecurityRadarBundle {
	if SecurityRadarHasLiveEvidence(bundle) {
		return bundle
	}
	bundle.PumpSybilRadar = insufficientEvidenceVerdict(bundle.PumpSybilRadar)
	bundle.RaydiumPoolGuardian = insufficientEvidenceVerdict(bundle.RaydiumPoolGuardian)
	bundle.WalletlessClaimShield = insufficientEvidenceVerdict(bundle.WalletlessClaimShield)
	bundle.CustomerSummary = SecurityRadarInsufficientEvidenceMessage
	bundle.CustomerRecommendation = "insufficient_evidence"
	if bundle.Metadata == nil {
		bundle.Metadata = map[string]any{}
	}
	bundle.Metadata["final_grade"] = "-"
	bundle.Metadata["final_risk_index"] = 0
	bundle.Metadata["final_risk_level"] = "unknown"
	bundle.Metadata["final_recommendation"] = "insufficient_evidence"
	bundle.Metadata["score_source"] = "none"
	bundle.Metadata["data_quality"] = "no_rpc_evidence"
	bundle.Metadata["evidence_status"] = "insufficient_evidence"
	return bundle
}

func EvidenceBackedFinalSecurityRadarVerdict(bundle SecurityRadarBundle) SecurityRadarFinalVerdict {
	bundle = EvidenceBackedSecurityRadarBundle(bundle)
	if !SecurityRadarHasLiveEvidence(bundle) {
		return SecurityRadarFinalVerdict{
			Grade:          "-",
			RiskIndex:      0,
			RiskLevel:      "unknown",
			Verdict:        SecurityRadarInsufficientEvidenceMessage,
			Recommendation: "insufficient_evidence",
			RuleVersion:    SecurityRadarRuleVersion,
			Signed:         false,
			Signature:      "",
		}
	}
	final := FinalSecurityRadarVerdict(bundle)
	final.Signed = true
	return final
}

func insufficientEvidenceVerdict(verdict SecurityRadarVerdict) SecurityRadarVerdict {
	if verdict.Signals == nil {
		verdict.Signals = map[string]any{}
	}
	verdict.Grade = "-"
	verdict.RiskIndex = 0
	verdict.RiskLevel = "unknown"
	verdict.Verdict = SecurityRadarInsufficientEvidenceMessage
	verdict.Recommendation = "insufficient_evidence"
	verdict.Signed = false
	verdict.Signature = ""
	verdict.Signals["score_source"] = "none"
	verdict.Signals["real_onchain_evidence"] = false
	verdict.Signals["data_quality"] = "no_rpc_evidence"
	verdict.Signals["evidence_status"] = "insufficient_evidence"
	verdict.Evidence = []string{SecurityRadarInsufficientEvidenceMessage}
	return verdict
}
