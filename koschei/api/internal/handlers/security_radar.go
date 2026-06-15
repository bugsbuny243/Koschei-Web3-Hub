package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type securityRadarInput struct {
	Target  string `json:"target"`
	Address string `json:"address"`
	Network string `json:"network"`
	Mode    string `json:"mode"`
}

func (h *Handler) SecurityRadarFeed(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": []any{}, "source": "koschei_security_radar", "provider": services.SecurityRadarProvider, "watch_mode": services.SecurityRadarWatchMode})
		return
	}
	items, err := services.NewSecurityRadarStore(h.DBRead).LatestVerdicts(r.Context(), 50)
	if isMissingRelation(err) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": []any{}, "source": "schema_pending", "provider": services.SecurityRadarProvider, "watch_mode": services.SecurityRadarWatchMode})
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, APICodeIntegrationError, "Radar feed unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": items, "source": "koschei_security_radar", "provider": services.SecurityRadarProvider, "watch_mode": services.SecurityRadarWatchMode})
}

func (h *Handler) SecurityRadarCheck(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized")
		return
	}
	claimEmail := normalizedClaimEmail(claims)
	if _, err := h.requirePremiumOutput(claims.Sub, claimEmail); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	var input securityRadarInput
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body")
		return
	}
	target := strings.TrimSpace(firstNonEmptyString(input.Target, input.Address))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required")
		return
	}
	if input.Network == "" {
		input.Network = "solana-mainnet"
	}
	bundle := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{Target: target, Network: input.Network, Mode: firstNonEmptyString(input.Mode, "manual_dashboard_check")})
	_ = h.saveSecurityRadarBundle(r.Context(), claims.Sub, "manual_check", bundle)
	if err := h.consumePremiumOutput(claims.Sub, claimEmail, "security_radar_check"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	h.logTool(claimEmail, "security_radar_check", "completed")
	h.trackEvent(claimEmail, "security_radar_check", r.URL.Path)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bundle": bundle, "final_verdict": services.FinalSecurityRadarVerdict(bundle)})
}

func (h *Handler) SecurityRiskBadge(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		address = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "address parameter is required"})
		return
	}
	network := strings.TrimSpace(r.URL.Query().Get("network"))
	if network == "" {
		network = "solana-mainnet"
	}
	bundle := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{Target: address, Network: network, Mode: "public_badge"})
	final := services.FinalSecurityRadarVerdict(bundle)
	if h != nil && h.DB != nil {
		_ = h.saveSecurityRadarBundle(r.Context(), "", "public_badge", bundle)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "address": address, "grade": final.Grade, "risk_index": final.RiskIndex, "risk_level": final.RiskLevel, "verdict": final.Verdict, "rule_version": final.RuleVersion, "signed": final.Signed, "signature": final.Signature})
}

func (h *Handler) OwnerRadarSummary(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "radar": map[string]any{}})
		return
	}
	metrics := map[string]any{"rule_version": services.SecurityRadarRuleVersion}
	count := func(key, query string) {
		var v int64
		if err := h.DBRead.QueryRowContext(r.Context(), query).Scan(&v); err == nil {
			metrics[key] = v
		}
	}
	count("radar_events", `SELECT count(*) FROM security_radar_events`)
	count("radar_verdicts", `SELECT count(*) FROM security_radar_verdicts`)
	count("badge_consumers", `SELECT count(DISTINCT target) FROM security_radar_events WHERE COALESCE(source,'')='public_badge'`)
	count("critical_risk_count", `SELECT count(*) FROM security_radar_verdicts WHERE risk_level='critical'`)
	count("high_risk_count", `SELECT count(*) FROM security_radar_verdicts WHERE risk_level='high'`)
	count("module_usage", `SELECT count(DISTINCT module_id) FROM security_radar_verdicts`)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "radar": metrics})
}

func (h *Handler) saveSecurityRadarBundle(ctx context.Context, userID, source string, bundle services.SecurityRadarBundle) error {
	if h == nil || h.DB == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	verdicts := []services.SecurityRadarVerdict{bundle.PumpSybilRadar, bundle.RaydiumPoolGuardian, bundle.WalletlessClaimShield}
	for _, verdict := range verdicts {
		if err := h.saveSecurityRadarVerdict(ctx, userID, source, verdict); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) saveSecurityRadarVerdict(ctx context.Context, userID, source string, verdict services.SecurityRadarVerdict) error {
	store := services.NewSecurityRadarStore(h.DB)
	eventID, err := store.InsertEvent(ctx, services.SecurityRadarEventRecord{
		ModuleID:   verdict.ModuleID,
		Target:     verdict.Target,
		TargetType: "token",
		Network:    verdict.Network,
		Signature:  verdict.Signature,
		EventType:  firstNonEmptyString(source, "manual_verdict"),
		Signals:    verdict.Signals,
		RawSummary: map[string]any{"source": source, "user_id": userID},
	})
	if err != nil {
		return err
	}
	_, err = store.InsertVerdict(ctx, services.SecurityRadarVerdictRecord{
		EventID:        eventID,
		ModuleID:       verdict.ModuleID,
		Target:         verdict.Target,
		TargetType:     "token",
		Network:        verdict.Network,
		Grade:          verdict.Grade,
		RiskIndex:      verdict.RiskIndex,
		RiskLevel:      verdict.RiskLevel,
		Verdict:        verdict.Verdict,
		Recommendation: verdict.Recommendation,
		Evidence:       verdict.Evidence,
		Signals:        verdict.Signals,
		RuleVersion:    verdict.RuleVersion,
		Signed:         verdict.Signed,
		Signature:      verdict.Signature,
	})
	return err
}

func jsonRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}
