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

type radarDBRow struct {
	Target         string
	TargetType     string
	ModuleID       string
	Signature      string
	RiskIndex      int
	RiskLevel      string
	Grade          string
	Verdict        string
	Recommendation string
	Evidence       json.RawMessage
	Signals        json.RawMessage
	RuleVersion    string
	CreatedAt      time.Time
}

func (h *Handler) SecurityRadarFeed(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": []any{}, "source": "deterministic_runtime"})
		return
	}
	limit := 50
	rows, err := h.DBRead.QueryContext(r.Context(), `
		SELECT target, COALESCE(target_type,''), module_id, COALESCE(signature,''), risk_index, risk_level, grade, verdict, recommendation, evidence, signals, rule_version, created_at
		FROM security_radar_verdicts
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if isMissingRelation(err) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": []any{}, "source": "schema_pending"})
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, APICodeIntegrationError, "Radar feed unavailable")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var row radarDBRow
		if err := rows.Scan(&row.Target, &row.TargetType, &row.ModuleID, &row.Signature, &row.RiskIndex, &row.RiskLevel, &row.Grade, &row.Verdict, &row.Recommendation, &row.Evidence, &row.Signals, &row.RuleVersion, &row.CreatedAt); err != nil {
			continue
		}
		items = append(items, map[string]any{"target": row.Target, "target_type": row.TargetType, "module_id": row.ModuleID, "signature": row.Signature, "risk_index": row.RiskIndex, "risk_level": row.RiskLevel, "grade": row.Grade, "verdict": row.Verdict, "recommendation": row.Recommendation, "evidence": jsonRaw(row.Evidence), "signals": jsonRaw(row.Signals), "rule_version": row.RuleVersion, "created_at": row.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": items, "source": "security_radar_verdicts"})
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
	count("badge_consumers", `SELECT count(DISTINCT target) FROM security_radar_events WHERE source='public_badge'`)
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
	evidence, _ := json.Marshal(verdict.Evidence)
	signals, _ := json.Marshal(verdict.Signals)
	_, _ = h.DB.ExecContext(ctx, `INSERT INTO security_radar_events (target, target_type, module_id, source, signature, signals, evidence, created_at, updated_at) VALUES ($1,'token',$2,$3,$4,$5::jsonb,$6::jsonb,now(),now()) ON CONFLICT DO NOTHING`, verdict.Target, verdict.ModuleID, nullIfEmpty(source), verdict.Signature, string(signals), string(evidence))
	_, err := h.DB.ExecContext(ctx, `INSERT INTO security_radar_verdicts (target, target_type, module_id, signature, risk_index, risk_level, grade, verdict, recommendation, evidence, signals, rule_version, user_id, source, created_at, updated_at) VALUES ($1,'token',$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11,NULLIF($12,''),NULLIF($13,''),now(),now()) ON CONFLICT (signature,module_id) DO UPDATE SET risk_index=EXCLUDED.risk_index, risk_level=EXCLUDED.risk_level, grade=EXCLUDED.grade, verdict=EXCLUDED.verdict, recommendation=EXCLUDED.recommendation, evidence=EXCLUDED.evidence, signals=EXCLUDED.signals, updated_at=now()`, verdict.Target, verdict.ModuleID, verdict.Signature, verdict.RiskIndex, verdict.RiskLevel, verdict.Grade, verdict.Verdict, verdict.Recommendation, string(evidence), string(signals), verdict.RuleVersion, userID, source)
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
