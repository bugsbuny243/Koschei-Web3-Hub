package handlers

import (
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) SecurityRadarExposureReport(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("target"), r.URL.Query().Get("address"), r.URL.Query().Get("mint")))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required")
		return
	}
	network := strings.TrimSpace(r.URL.Query().Get("network"))
	if network == "" {
		network = "solana-mainnet"
	}
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = "exposure_report"
	}

	analysis := services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: target, Network: network, Mode: mode})
	bundle := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	final := services.ArvisFinalFromBundle(bundle)
	arms := services.ArvisArmsFromBundle(bundle)
	if !services.SecurityRadarHasLiveEvidence(bundle) || !final.Signed {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "error": "real_data_unavailable", "message": services.SecurityRadarInsufficientEvidenceMessage,
			"target": target, "network": network, "final_verdict": final,
		})
		return
	}
	if h != nil && h.DB != nil {
		_ = h.saveSecurityRadarBundle(r.Context(), "", "exposure_report", bundle)
	}

	report := buildSecurityRadarExposureReport(target, network, final, arms, bundle.Metadata)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "report": report, "final_verdict": final, "arms": arms})
}

func buildSecurityRadarExposureReport(target, network string, final services.SecurityRadarFinalVerdict, arms []services.SecurityRadarVerdict, metadata map[string]any) map[string]any {
	verified := 0
	unavailable := 0
	maxRisk := 0
	maxModule := ""
	evidence := []string{}
	for _, arm := range arms {
		if securityRadarArmVerified(arm) {
			verified++
			if len(evidence) < 10 {
				evidence = append(evidence, arm.Evidence...)
			}
		} else {
			unavailable++
		}
		if arm.RiskIndex > maxRisk {
			maxRisk = arm.RiskIndex
			maxModule = arm.ModuleID
		}
	}
	return map[string]any{
		"schema_version": "koschei-exposure-report-v1",
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"target": target,
		"network": network,
		"verdict": final,
		"summary": map[string]any{
			"verified_arm_count": verified,
			"unavailable_arm_count": unavailable,
			"max_risk_index": maxRisk,
			"max_risk_module": maxModule,
			"rule_version": final.RuleVersion,
			"signed": final.Signed,
		},
		"sections": map[string]any{
			"authority": exposureSectionFromArm(arms, services.ModuleTokenAuthorityScanner, []string{"mint_authority_present", "freeze_authority_present", "account_owner"}),
			"holder_concentration": exposureSectionFromArm(arms, services.ModuleHolderConcentration, []string{"largest_holder_percentage", "top_10_holder_percentage", "largest_accounts", "token_supply"}),
			"intelligence_graph": exposureSectionFromArm(arms, services.ModuleIntelligenceGraph, []string{"account_owner", "latest_signature", "largest_accounts"}),
			"sniper_timing": exposureSectionFromArm(arms, services.ModuleSniperTimingDetector, []string{"recent_signature_count", "signature_window_seconds", "failed_signature_count", "scope_note"}),
			"program_relation": exposureSectionFromArm(arms, services.ModuleProgramRelationScan, []string{"account_owner", "program_id", "account_executable"}),
			"liquidity": exposureSectionFromArm(arms, services.ModuleLiquidityMovement, []string{"pool", "reserve", "liquidity"}),
		},
		"evidence": firstExposureEvidence(evidence, 10),
		"metadata": metadata,
		"disclaimer": "This is evidence-backed on-chain risk analysis, not an accusation or financial advice.",
		"signature": final.Signature,
	}
}

func exposureSectionFromArm(arms []services.SecurityRadarVerdict, moduleID string, signalKeys []string) map[string]any {
	for _, arm := range arms {
		if arm.ModuleID != moduleID {
			continue
		}
		signals := map[string]any{}
		for _, key := range signalKeys {
			if value, ok := arm.Signals[key]; ok {
				signals[key] = value
			}
		}
		return map[string]any{"module_id": arm.ModuleID, "module": arm.Module, "risk_index": arm.RiskIndex, "risk_level": arm.RiskLevel, "verified": securityRadarArmVerified(arm), "signals": signals, "evidence": firstExposureEvidence(arm.Evidence, 5)}
	}
	return map[string]any{"module_id": moduleID, "verified": false, "evidence": []string{}}
}

func securityRadarArmVerified(arm services.SecurityRadarVerdict) bool {
	if !arm.Signed || arm.Signals == nil {
		return false
	}
	for _, key := range []string{"verified_evidence", "real_onchain_evidence", "real_offchain_evidence"} {
		if value, _ := arm.Signals[key].(bool); value {
			return true
		}
	}
	return false
}

func firstExposureEvidence(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}
