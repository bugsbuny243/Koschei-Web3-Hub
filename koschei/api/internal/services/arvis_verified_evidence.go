package services

import "strings"

func SecurityRadarVerdictHasVerifiedEvidence(verdict SecurityRadarVerdict) bool {
	if verdict.EvidenceVerified {
		return true
	}
	return securityRadarSignalsHaveVerifiedEvidence(verdict.Signals)
}

func securityRadarSignalsHaveVerifiedEvidence(signals map[string]any) bool {
	if signals == nil {
		return false
	}
	if value, _ := signals["verified_evidence"].(bool); value {
		return true
	}
	if value, _ := signals["real_onchain_evidence"].(bool); value {
		return true
	}
	if value, _ := signals["real_offchain_evidence"].(bool); value {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(arvisSignalString(signals, "evidence_status")))
	return status == "verified" || strings.HasPrefix(status, "verified_")
}

func verifiedArvisEvidenceCount(arms []SecurityRadarVerdict) int {
	count := 0
	for _, arm := range arms {
		if arm.ModuleID == ModuleFinalVerdictEngine {
			continue
		}
		if SecurityRadarVerdictHasVerifiedEvidence(arm) {
			count++
		}
	}
	return count
}
