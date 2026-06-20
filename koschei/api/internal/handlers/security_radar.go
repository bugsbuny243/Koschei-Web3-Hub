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

type securityRadarInput struct {
	Target  string `json:"target"`
	Address string `json:"address"`
	Network string `json:"network"`
	Mode    string `json:"mode"`
}

func (h *Handler) SecurityRadarFeed(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("graph")), "1") || strings.TrimSpace(r.URL.Query().Get("verdict_id")) != "" {
		h.SecurityRadarGraph(w, r)
		return
	}
	base := map[string]any{"ok": true, "source": "koschei_security_radar", "provider": services.SecurityRadarProvider, "watch_mode": services.SecurityRadarWatchMode, "architecture_arm_count": 14}
	if h == nil || h.DBRead == nil {
		base["items"] = []any{}
		base["stream"] = map[string]any{"enabled": streamEnvEnabled(), "wss_configured": streamWSSConfigured(), "pipeline_status": "database_unavailable"}
		base["timeline"] = []any{}
		writeJSON(w, http.StatusOK, base)
		return
	}
	items, err := services.NewSecurityRadarStore(h.DBRead).LatestVerdicts(r.Context(), 100)
	if isMissingRelation(err) {
		base["items"] = []any{}
		base["source"] = "schema_pending"
		base["stream"] = map[string]any{"enabled": streamEnvEnabled(), "wss_configured": streamWSSConfigured(), "schema_pending": true, "pipeline_status": "schema_pending"}
		base["timeline"] = []any{}
		writeJSON(w, http.StatusOK, base)
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, APICodeIntegrationError, "Radar feed unavailable")
		return
	}
	verified := make([]services.SecurityRadarVerdictRecord, 0, len(items))
	for _, item := range items {
		if item.Signed && radarSignalsVerified(item.Signals) && item.ModuleID == services.ModuleFinalVerdictEngine {
			verified = append(verified, item)
		}
	}
	base["items"] = verified
	base["stream"] = h.securityRadarStreamStats(r.Context())
	base["timeline"] = h.securityRadarStreamTimeline(r.Context(), 14)
	writeJSON(w, http.StatusOK, base)
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
	mode := firstNonEmptyString(input.Mode, "manual_dashboard_check")
	services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "radar_check_requested", "customer", "info", map[string]any{"network": input.Network, "mode": mode}))
	analysis := services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: target, Network: input.Network, Mode: mode})
	bundle := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	final := services.ArvisFinalFromBundle(bundle)
	arms := services.ArvisArmsFromBundle(bundle)
	if !services.SecurityRadarHasLiveEvidence(bundle) || !final.Signed {
		services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "radar_check_no_evidence", "customer", "warning", map[string]any{"network": input.Network, "target": target}))
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "real_data_unavailable", "message": services.SecurityRadarInsufficientEvidenceMessage, "bundle": bundle, "arms": arms, "final_verdict": final, "charged": false})
		return
	}
	_ = h.saveSecurityRadarBundle(r.Context(), claims.Sub, "manual_check", bundle)
	if err := h.consumePremiumOutput(claims.Sub, claimEmail, "security_radar_check"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	h.logTool(claimEmail, "security_radar_check", "completed")
	h.trackEvent(claimEmail, "security_radar_check", r.URL.Path)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bundle": bundle, "arms": arms, "final_verdict": final})
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
	analysis := services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: address, Network: network, Mode: "public_badge"})
	bundle := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	final := services.ArvisFinalFromBundle(bundle)
	if !services.SecurityRadarHasLiveEvidence(bundle) || !final.Signed {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "real_data_unavailable", "message": services.SecurityRadarInsufficientEvidenceMessage, "address": address, "grade": final.Grade, "risk_index": final.RiskIndex, "risk_level": final.RiskLevel, "signed": false})
		return
	}
	if h != nil && h.DB != nil {
		_ = h.saveSecurityRadarBundle(r.Context(), "", "public_badge", bundle)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "address": address, "grade": final.Grade, "risk_index": final.RiskIndex, "risk_level": final.RiskLevel, "verdict": final.Verdict, "recommendation": final.Recommendation, "rule_version": final.RuleVersion, "signed": final.Signed, "signature": final.Signature, "verified_arm_count": bundle.Metadata["verified_arm_count"]})
}

func (h *Handler) OwnerRadarSummary(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "radar": map[string]any{}})
		return
	}
	metrics := map[string]any{"rule_version": services.SecurityRadarRuleVersion, "architecture_arm_count": 14, "sbx1_stream_enabled": streamEnvEnabled(), "sbx1_wss_configured": streamWSSConfigured()}
	count := func(key, query string) {
		var value int64
		if err := h.DBRead.QueryRowContext(r.Context(), query).Scan(&value); err == nil {
			metrics[key] = value
		}
	}
	text := func(key, query string) {
		var value string
		if err := h.DBRead.QueryRowContext(r.Context(), query).Scan(&value); err == nil {
			metrics[key] = value
		}
	}
	verifiedSQL := radarVerifiedEvidenceSQL()
	count("radar_events", `SELECT count(*) FROM security_radar_events`)
	count("radar_verdicts", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND `+verifiedSQL)
	count("badge_consumers", `SELECT count(DISTINCT target) FROM security_radar_events WHERE COALESCE(source,'')='public_badge'`)
	count("critical_risk_count", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND risk_level='critical' AND signed=true AND `+verifiedSQL)
	count("high_risk_count", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND risk_level='high' AND signed=true AND `+verifiedSQL)
	count("module_usage", `SELECT count(DISTINCT module_id) FROM security_radar_verdicts WHERE module_id <> 'final_verdict_engine' AND signed=true AND `+verifiedSQL)
	count("sbx1_stream_events", `SELECT count(*) FROM security_radar_stream_events`)
	count("sbx1_recognized_events", `SELECT count(*) FROM security_radar_stream_events WHERE module_id <> 'unknown'`)
	text("sbx1_last_event_at", `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_stream_events`)
	h.addArvisProcessingMetrics(r.Context(), metrics)
	metrics["pipeline_status"] = arvisPipelineStatus(metrics)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "radar": metrics})
}

func (h *Handler) securityRadarStreamStats(ctx context.Context) map[string]any {
	metrics := map[string]any{"enabled": streamEnvEnabled(), "wss_configured": streamWSSConfigured(), "architecture_arm_count": 14}
	if h == nil || h.DBRead == nil {
		metrics["pipeline_status"] = "database_unavailable"
		return metrics
	}
	count := func(key, query string) {
		var value int64
		if err := h.DBRead.QueryRowContext(ctx, query).Scan(&value); err == nil {
			metrics[key] = value
		}
	}
	text := func(key, query string) {
		var value string
		if err := h.DBRead.QueryRowContext(ctx, query).Scan(&value); err == nil {
			metrics[key] = value
		}
	}
	verifiedSQL := radarVerifiedEvidenceSQL()
	count("raw_stream_events", `SELECT count(*) FROM security_radar_stream_events`)
	count("recognized_events", `SELECT count(*) FROM security_radar_stream_events WHERE module_id <> 'unknown'`)
	count("enriched_mints", `SELECT count(*) FROM security_radar_stream_events WHERE evidence_quality='transaction_enriched_mint'`)
	count("visible_verdicts", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND `+verifiedSQL)
	count("runtime_engines", `SELECT count(DISTINCT module_id) FROM security_radar_verdicts WHERE module_id <> 'final_verdict_engine' AND signed=true AND `+verifiedSQL)
	text("last_stream_event_at", `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_stream_events`)
	text("last_stream_signature", `SELECT COALESCE(signature,'') FROM security_radar_stream_events WHERE signature IS NOT NULL ORDER BY created_at DESC LIMIT 1`)
	text("last_stream_module", `SELECT COALESCE(module_id,'') FROM security_radar_stream_events ORDER BY created_at DESC LIMIT 1`)
	h.addArvisProcessingMetrics(ctx, metrics)
	metrics["pipeline_status"] = arvisPipelineStatus(metrics)
	return metrics
}

func (h *Handler) addArvisProcessingMetrics(ctx context.Context, metrics map[string]any) {
	if h == nil || h.DBRead == nil || metrics == nil {
		return
	}
	count := func(key, status string) {
		var value int64
		err := h.DBRead.QueryRowContext(ctx, `SELECT count(*) FROM arvis_stream_processing WHERE status=$1`, status).Scan(&value)
		if err == nil {
			metrics[key] = value
		}
	}
	count("processing_pending", "pending")
	count("processing_active", "processing")
	count("processing_completed", "completed")
	count("processing_insufficient", "insufficient_evidence")
	count("processing_failed", "failed")
	var lastProcessed string
	if err := h.DBRead.QueryRowContext(ctx, `SELECT COALESCE(max(processed_at)::text,'') FROM arvis_stream_processing`).Scan(&lastProcessed); err == nil {
		metrics["last_processed_at"] = lastProcessed
	}
}

func arvisPipelineStatus(metrics map[string]any) string {
	raw := metricInt64(metrics, "raw_stream_events")
	active := metricInt64(metrics, "processing_active")
	completed := metricInt64(metrics, "processing_completed")
	failed := metricInt64(metrics, "processing_failed")
	if active > 0 {
		return "processing"
	}
	if completed > 0 && failed == 0 {
		return "healthy"
	}
	if completed > 0 && failed > 0 {
		return "degraded"
	}
	if failed > 0 {
		return "degraded"
	}
	if raw > 0 {
		return "waiting_for_enriched_targets"
	}
	return "waiting_for_stream"
}

func metricInt64(metrics map[string]any, key string) int64 {
	if metrics == nil {
		return 0
	}
	switch value := metrics[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func radarVerifiedEvidenceSQL() string {
	return `(COALESCE(signals->>'verified_evidence','false')='true' OR COALESCE(signals->>'real_onchain_evidence','false')='true' OR COALESCE(signals->>'real_offchain_evidence','false')='true')`
}

func radarSignalsVerified(signals map[string]any) bool {
	if signals == nil {
		return false
	}
	for _, key := range []string{"verified_evidence", "real_onchain_evidence", "real_offchain_evidence"} {
		if value, _ := signals[key].(bool); value {
			return true
		}
	}
	return false
}

func (h *Handler) securityRadarStreamTimeline(ctx context.Context, limit int) []map[string]any {
	out := []map[string]any{}
	if h == nil || h.DBRead == nil {
		return out
	}
	if limit <= 0 || limit > 30 {
		limit = 14
	}
	rows, err := h.DBRead.QueryContext(ctx, `
		SELECT module_id, event_type, COALESCE(target,''), target_type, COALESCE(signature,''), COALESCE(slot,0), COALESCE(program_id,''), evidence_quality, created_at::text
		FROM security_radar_stream_events
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var moduleID, eventType, target, targetType, signature, programID, evidenceQuality, createdAt string
		var slot sql.NullInt64
		if err := rows.Scan(&moduleID, &eventType, &target, &targetType, &signature, &slot, &programID, &evidenceQuality, &createdAt); err != nil {
			continue
		}
		label, meaning := arvisTimelineLabel(moduleID, eventType, evidenceQuality, target)
		out = append(out, map[string]any{"module_id": moduleID, "event_type": eventType, "target": target, "target_type": targetType, "signature": signature, "slot": slot.Int64, "program_id": programID, "evidence_quality": evidenceQuality, "created_at": createdAt, "label": label, "meaning": meaning})
	}
	return out
}

func arvisTimelineLabel(moduleID, eventType, evidenceQuality, target string) (string, string) {
	m := strings.ToLower(moduleID + " " + eventType + " " + evidenceQuality)
	targetLabel := "a Solana event"
	if strings.TrimSpace(target) != "" {
		targetLabel = "target " + shortRadarTarget(target)
	}
	if strings.Contains(m, "raydium") {
		return "Raydium pool activity", "Arvıs detected pool/liquidity-related activity for " + targetLabel + "."
	}
	if strings.Contains(m, "pump") {
		return "Pump.fun launch activity", "Arvıs detected launch/buyer-flow activity for " + targetLabel + "."
	}
	if strings.Contains(m, "mint") || strings.Contains(m, "token") {
		return "Token authority / mint activity", "Arvıs detected token or mint-related activity for " + targetLabel + "."
	}
	if strings.Contains(m, "transaction_enriched") {
		return "Enriched mint resolved", "Arvıs resolved a transaction into a usable mint target."
	}
	return "Raw Solana stream event", "Arvıs captured a live Solana event; it stays internal until enough evidence exists."
}

func shortRadarTarget(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 18 {
		return v
	}
	return v[:8] + "…" + v[len(v)-6:]
}

func streamEnvEnabled() bool {
	v := strings.TrimSpace(firstNonEmptyString(os.Getenv("RADAR_STREAM_ENABLED"), os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED")))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on") || strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_SOLANA_WATCH_MODE")), "stream")
}

func streamWSSConfigured() bool {
	return strings.TrimSpace(firstNonEmptyString(os.Getenv("SOLANA_WSS_URL"), os.Getenv("ALCHEMY_SOLANA_WSS_URL"), os.Getenv("HELIUS_SOLANA_WSS_URL"), os.Getenv("QUICKNODE_SOLANA_WSS_URL"), os.Getenv("PUMPPORTAL_DATA_WS"), os.Getenv("ALCHEMY_API_KEY"))) != ""
}

func (h *Handler) saveSecurityRadarBundle(ctx context.Context, userID, source string, bundle services.SecurityRadarBundle) error {
	if h == nil || h.DB == nil || !services.SecurityRadarHasLiveEvidence(bundle) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	verdicts := services.ArvisArmsFromBundle(bundle)
	if len(verdicts) == 0 {
		verdicts = []services.SecurityRadarVerdict{bundle.PumpSybilRadar, bundle.RaydiumPoolGuardian, bundle.WalletlessClaimShield}
	}
	for _, verdict := range verdicts {
		if !services.SecurityRadarVerdictHasVerifiedEvidence(verdict) {
			continue
		}
		if err := h.saveSecurityRadarVerdict(ctx, userID, source, verdict); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) saveSecurityRadarVerdict(ctx context.Context, userID, source string, verdict services.SecurityRadarVerdict) error {
	if !services.SecurityRadarVerdictHasVerifiedEvidence(verdict) {
		return nil
	}
	targetType := "token"
	if onchain, _ := verdict.Signals["real_onchain_evidence"].(bool); !onchain {
		if offchain, _ := verdict.Signals["real_offchain_evidence"].(bool); offchain {
			targetType = "url"
		}
	}
	store := services.NewSecurityRadarStore(h.DB)
	eventID, err := store.InsertEvent(ctx, services.SecurityRadarEventRecord{ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: targetType, Network: verdict.Network, Signature: verdict.Signature, EventType: firstNonEmptyString(source, "manual_verdict"), Signals: verdict.Signals, RawSummary: map[string]any{"source": source, "user_id": userID}, Source: firstNonEmptyString(source, "manual_check")})
	if err != nil {
		return err
	}
	_, err = store.InsertVerdict(ctx, services.SecurityRadarVerdictRecord{EventID: eventID, ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: targetType, Network: verdict.Network, Grade: verdict.Grade, RiskIndex: verdict.RiskIndex, RiskLevel: verdict.RiskLevel, Verdict: verdict.Verdict, Recommendation: verdict.Recommendation, Evidence: verdict.Evidence, Signals: verdict.Signals, RuleVersion: verdict.RuleVersion, Signed: verdict.Signed, Signature: verdict.Signature, Source: firstNonEmptyString(source, "manual_check")})
	return err
}

func jsonRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}
