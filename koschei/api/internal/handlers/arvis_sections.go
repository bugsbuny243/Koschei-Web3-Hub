package handlers

import "koschei/api/internal/services"

func arvisSections(arms []services.SecurityRadarVerdict, final services.SecurityRadarFinalVerdict) map[string]any {
	sections := map[string]any{
		"final_verdict": map[string]any{
			"grade":          final.Grade,
			"risk_index":     final.RiskIndex,
			"risk_level":     final.RiskLevel,
			"verdict":        final.Verdict,
			"recommendation": final.Recommendation,
			"rule_version":   final.RuleVersion,
			"signed":         final.Signed,
			"signature":      final.Signature,
		},
		"arvis_arms": arms,
	}
	verified := 0
	for _, arm := range arms {
		sections[arm.ModuleID] = arm
		if arm.ModuleID == services.ModuleFinalVerdictEngine || !arm.Signed || arm.Signals == nil {
			continue
		}
		if ok, _ := arm.Signals["real_onchain_evidence"].(bool); ok {
			verified++
		}
	}
	sections["architecture_arm_count"] = 14
	sections["verified_arm_count"] = verified
	return sections
}
