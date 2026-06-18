package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type securityRadarInput struct { Target string `json:"target"`; Address string `json:"address"`; Network string `json:"network"`; Mode string `json:"mode"` }

func (h *Handler) SecurityRadarFeed(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("graph")), "1") || strings.TrimSpace(r.URL.Query().Get("verdict_id")) != "" { h.SecurityRadarGraph(w, r); return }
	base := map[string]any{"ok": true, "source": "koschei_security_radar", "provider": services.SecurityRadarProvider, "watch_mode": services.SecurityRadarWatchMode}
	if h == nil || h.DBRead == nil { base["items"] = []any{}; base["stream"] = map[string]any{"enabled": streamEnvEnabled(), "wss_configured": streamWSSConfigured()}; base["timeline"] = []any{}; writeJSON(w, http.StatusOK, base); return }
	items, err := services.NewSecurityRadarStore(h.DBRead).LatestVerdicts(r.Context(), 50)
	if isMissingRelation(err) { base["items"] = []any{}; base["source"] = "schema_pending"; base["stream"] = map[string]any{"enabled": streamEnvEnabled(), "wss_configured": streamWSSConfigured(), "schema_pending": true}; base["timeline"] = []any{}; writeJSON(w, http.StatusOK, base); return }
	if err != nil { writeAPIError(w, http.StatusInternalServerError, APICodeIntegrationError, "Radar feed unavailable"); return }
	base["items"] = items
	base["stream"] = h.securityRadarStreamStats(r.Context())
	base["timeline"] = h.securityRadarStreamTimeline(r.Context(), 14)
	writeJSON(w, http.StatusOK, base)
}

func (h *Handler) SecurityRadarCheck(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context()); if !ok { writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized"); return }
	claimEmail := normalizedClaimEmail(claims)
	if _, err := h.requirePremiumOutput(claims.Sub, claimEmail); err != nil { writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse()); return }
	var input securityRadarInput
	if err := decodeJSON(r, &input); err != nil { writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body"); return }
	target := strings.TrimSpace(firstNonEmptyString(input.Target, input.Address))
	if target == "" { writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required"); return }
	if input.Network == "" { input.Network = "solana-mainnet" }
	services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "radar_check_requested", "customer", "info", map[string]any{"network": input.Network, "mode": firstNonEmptyString(input.Mode, "manual_dashboard_check")}))
	bundle := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{Target: target, Network: input.Network, Mode: firstNonEmptyString(input.Mode, "manual_dashboard_check")})
	_ = h.saveSecurityRadarBundle(r.Context(), claims.Sub, "manual_check", bundle)
	if err := h.consumePremiumOutput(claims.Sub, claimEmail, "security_radar_check"); err != nil { writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse()); return }
	h.logTool(claimEmail, "security_radar_check", "completed"); h.trackEvent(claimEmail, "security_radar_check", r.URL.Path)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bundle": bundle, "final_verdict": services.FinalSecurityRadarVerdict(bundle)})
}

func (h *Handler) SecurityRiskBadge(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address")); if address == "" { address = strings.TrimSpace(r.URL.Query().Get("token")) }
	if address == "" { writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "address parameter is required"}); return }
	network := strings.TrimSpace(r.URL.Query().Get("network")); if network == "" { network = "solana-mainnet" }
	bundle := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{Target: address, Network: network, Mode: "public_badge"}); final := services.FinalSecurityRadarVerdict(bundle)
	if h != nil && h.DB != nil { _ = h.saveSecurityRadarBundle(r.Context(), "", "public_badge", bundle) }
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "address": address, "grade": final.Grade, "risk_index": final.RiskIndex, "risk_level": final.RiskLevel, "verdict": final.Verdict, "rule_version": final.RuleVersion, "signed": final.Signed, "signature": final.Signature})
}

func (h *Handler) OwnerRadarSummary(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil { writeJSON(w, http.StatusOK, map[string]any{"ok": true, "radar": map[string]any{}}); return }
	metrics := map[string]any{"rule_version": services.SecurityRadarRuleVersion, "sbx1_stream_enabled": streamEnvEnabled(), "sbx1_wss_configured": streamWSSConfigured()}
	count := func(key, query string) { var v int64; if err := h.DBRead.QueryRowContext(r.Context(), query).Scan(&v); err == nil { metrics[key] = v } }
	text := func(key, query string) { var v string; if err := h.DBRead.QueryRowContext(r.Context(), query).Scan(&v); err == nil { metrics[key] = v } }
	count("radar_events", `SELECT count(*) FROM security_radar_events`); count("radar_verdicts", `SELECT count(*) FROM security_radar_verdicts`); count("badge_consumers", `SELECT count(DISTINCT target) FROM security_radar_events WHERE COALESCE(source,'')='public_badge'`); count("critical_risk_count", `SELECT count(*) FROM security_radar_verdicts WHERE risk_level='critical' AND module_id <> 'walletless_claim_shield'`); count("high_risk_count", `SELECT count(*) FROM security_radar_verdicts WHERE risk_level='high' AND module_id <> 'walletless_claim_shield'`); count("module_usage", `SELECT count(DISTINCT module_id) FROM security_radar_verdicts WHERE module_id <> 'walletless_claim_shield'`)
	count("sbx1_stream_events", `SELECT count(*) FROM security_radar_stream_events`); count("sbx1_recognized_events", `SELECT count(*) FROM security_radar_stream_events WHERE module_id <> 'unknown'`); text("sbx1_last_event_at", `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_stream_events`)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "radar": metrics})
}

func (h *Handler) securityRadarStreamStats(ctx context.Context) map[string]any {
	metrics := map[string]any{"enabled": streamEnvEnabled(), "wss_configured": streamWSSConfigured()}
	if h == nil || h.DBRead == nil { return metrics }
	count := func(key, query string) { var v int64; if err := h.DBRead.QueryRowContext(ctx, query).Scan(&v); err == nil { metrics[key] = v } }
	text := func(key, query string) { var v string; if err := h.DBRead.QueryRowContext(ctx, query).Scan(&v); err == nil { metrics[key] = v } }
	count("raw_stream_events", `SELECT count(*) FROM security_radar_stream_events`)
	count("recognized_events", `SELECT count(*) FROM security_radar_stream_events WHERE module_id <> 'unknown'`)
	count("enriched_mints", `SELECT count(*) FROM security_radar_stream_events WHERE evidence_quality='transaction_enriched_mint'`)
	count("visible_verdicts", `SELECT count(*) FROM security_radar_verdicts WHERE module_id <> 'walletless_claim_shield'`)
	text("last_stream_event_at", `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_stream_events`)
	text("last_stream_signature", `SELECT COALESCE(signature,'') FROM security_radar_stream_events WHERE signature IS NOT NULL ORDER BY created_at DESC LIMIT 1`)
	text("last_stream_module", `SELECT COALESCE(module_id,'') FROM security_radar_stream_events ORDER BY created_at DESC LIMIT 1`)
	return metrics
}

func (h *Handler) securityRadarStreamTimeline(ctx context.Context, limit int) []map[string]any {
	out := []map[string]any{}
	if h == nil || h.DBRead == nil { return out }
	if limit <= 0 || limit > 30 { limit = 14 }
	rows, err := h.DBRead.QueryContext(ctx, `
		SELECT module_id, event_type, COALESCE(target,''), target_type, COALESCE(signature,''), COALESCE(slot,0), COALESCE(program_id,''), evidence_quality, created_at::text
		FROM security_radar_stream_events
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil { return out }
	defer rows.Close()
	for rows.Next() {
		var moduleID, eventType, target, targetType, signature, programID, evidenceQuality, createdAt string
		var slot sql.NullInt64
		if err := rows.Scan(&moduleID, &eventType, &target, &targetType, &signature, &slot, &programID, &evidenceQuality, &createdAt); err != nil { continue }
		label, meaning := arvisTimelineLabel(moduleID, eventType, evidenceQuality, target)
		out = append(out, map[string]any{"module_id": moduleID, "event_type": eventType, "target": target, "target_type": targetType, "signature": signature, "slot": slot.Int64, "program_id": programID, "evidence_quality": evidenceQuality, "created_at": createdAt, "label": label, "meaning": meaning})
	}
	return out
}

func arvisTimelineLabel(moduleID, eventType, evidenceQuality, target string) (string, string) {
	m := strings.ToLower(moduleID + " " + eventType + " " + evidenceQuality)
	targetLabel := "a Solana event"
	if strings.TrimSpace(target) != "" { targetLabel = "target " + shortRadarTarget(target) }
	if strings.Contains(m, "raydium") { return "Raydium pool activity", "Arvıs detected pool/liquidity-related activity for " + targetLabel + "." }
	if strings.Contains(m, "pump") { return "Pump.fun launch activity", "Arvıs detected launch/buyer-flow activity for " + targetLabel + "." }
	if strings.Contains(m, "mint") || strings.Contains(m, "token") { return "Token authority / mint activity", "Arvıs detected token or mint-related activity for " + targetLabel + "." }
	if strings.Contains(m, "transaction_enriched") { return "Enriched mint resolved", "Arvıs resolved a transaction into a usable mint target." }
	return "Raw Solana stream event", "Arvıs captured a live Solana event; it stays internal until enough evidence exists."
}

func shortRadarTarget(v string) string { v = strings.TrimSpace(v); if len(v) <= 18 { return v }; return v[:8] + "…" + v[len(v)-6:] }

func streamEnvEnabled() bool {
	v := strings.TrimSpace(firstNonEmptyString(os.Getenv("RADAR_STREAM_ENABLED"), os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED")))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on") || strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_SOLANA_WATCH_MODE")), "stream")
}

func streamWSSConfigured() bool { return strings.TrimSpace(firstNonEmptyString(os.Getenv("SOLANA_WSS_URL"), os.Getenv("ALCHEMY_SOLANA_WSS_URL"), os.Getenv("HELIUS_SOLANA_WSS_URL"), os.Getenv("QUICKNODE_SOLANA_WSS_URL"), os.Getenv("PUMPPORTAL_DATA_WS"), os.Getenv("ALCHEMY_API_KEY"))) != "" }

func (h *Handler) saveSecurityRadarBundle(ctx context.Context, userID, source string, bundle services.SecurityRadarBundle) error {
	if h == nil || h.DB == nil { return nil }
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second); defer cancel()
	verdicts := []services.SecurityRadarVerdict{bundle.PumpSybilRadar, bundle.RaydiumPoolGuardian}
	for _, verdict := range verdicts { if err := h.saveSecurityRadarVerdict(ctx, userID, source, verdict); err != nil { return err } }
	return nil
}

func (h *Handler) saveSecurityRadarVerdict(ctx context.Context, userID, source string, verdict services.SecurityRadarVerdict) error {
	store := services.NewSecurityRadarStore(h.DB)
	eventID, err := store.InsertEvent(ctx, services.SecurityRadarEventRecord{ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: "token", Network: verdict.Network, Signature: verdict.Signature, EventType: firstNonEmptyString(source, "manual_verdict"), Signals: verdict.Signals, RawSummary: map[string]any{"source": source, "user_id": userID}, Source: firstNonEmptyString(source, "manual_check")})
	if err != nil { return err }
	_, err = store.InsertVerdict(ctx, services.SecurityRadarVerdictRecord{EventID: eventID, ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: "token", Network: verdict.Network, Grade: verdict.Grade, RiskIndex: verdict.RiskIndex, RiskLevel: verdict.RiskLevel, Verdict: verdict.Verdict, Recommendation: verdict.Recommendation, Evidence: verdict.Evidence, Signals: verdict.Signals, RuleVersion: verdict.RuleVersion, Signed: verdict.Signed, Signature: verdict.Signature, Source: firstNonEmptyString(source, "manual_check")})
	return err
}

func jsonRaw(raw json.RawMessage) any { if len(raw) == 0 { return nil }; var v any; if err := json.Unmarshal(raw, &v); err != nil { return string(raw) }; return v }
