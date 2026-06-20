package services

func SecurityRadarVerdictHasVerifiedEvidence(verdict SecurityRadarVerdict) bool {
	if !verdict.Signed || verdict.Signals == nil {
		return false
	}
	if value, _ := verdict.Signals["verified_evidence"].(bool); value {
		return true
	}
	if value, _ := verdict.Signals["real_onchain_evidence"].(bool); value {
		return true
	}
	if value, _ := verdict.Signals["real_offchain_evidence"].(bool); value {
		return true
	}
	return false
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
