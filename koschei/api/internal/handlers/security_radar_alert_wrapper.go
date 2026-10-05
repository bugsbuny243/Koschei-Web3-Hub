package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"koschei/api/internal/alerts"
	"koschei/api/internal/cryptobrief"
)

const maxSecurityRadarAlertBody = 1 << 20

// SecurityRadarCheckWithAlerts preserves the existing investigation response
// contract. Signed evidence-ready medium/high/critical verdicts still create
// durable security alerts. Separately, every successful customer scan may be
// queued to that customer's explicitly paired Telegram channel.
func (h *Handler) SecurityRadarCheckWithAlerts(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, maxSecurityRadarAlertBody+1))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body")
		return
	}
	if len(rawBody) > maxSecurityRadarAlertBody {
		writeAPIError(w, http.StatusRequestEntityTooLarge, APICodeInvalidInput, "Request body is too large")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	var input securityRadarInput
	_ = json.Unmarshal(rawBody, &input)
	target := strings.TrimSpace(firstNonEmptyString(input.Target, input.Address))

	recorder := httptest.NewRecorder()
	h.SecurityRadarCheck(recorder, r)
	result := recorder.Result()
	defer result.Body.Close()
	responseBody, _ := io.ReadAll(result.Body)

	if result.StatusCode >= 200 && result.StatusCode < 300 && h != nil && h.DB != nil {
		var envelope map[string]any
		if json.Unmarshal(responseBody, &envelope) == nil {
			changed := false
			if alertID := h.emitARVISVerdictAlert(r, target, envelope); alertID != "" {
				envelope["alert_event_id"] = alertID
				changed = true
			}
			if h.queueARVISCustomerTelegram(r, target, envelope) {
				envelope["telegram_delivery_queued"] = true
				changed = true
			}
			if changed {
				if encoded, marshalErr := json.Marshal(envelope); marshalErr == nil {
					responseBody = encoded
				}
			}
		}
	}

	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Del("Content-Length")
	w.WriteHeader(result.StatusCode)
	_, _ = w.Write(responseBody)
}

func (h *Handler) emitARVISVerdictAlert(r *http.Request, target string, envelope map[string]any) string {
	if r == nil || h == nil || h.DB == nil {
		return ""
	}
	status := strings.ToLower(strings.TrimSpace(stringFromMap(envelope, "status")))
	hasEvidence, _ := envelope["has_live_evidence"].(bool)
	final, _ := envelope["final_verdict"].(map[string]any)
	if status != "ready" || !hasEvidence || final == nil {
		return ""
	}
	signed, _ := final["signed"].(bool)
	if !signed {
		return ""
	}
	riskLevel := strings.ToLower(strings.TrimSpace(stringFromMap(final, "risk_level")))
	if riskLevel != "medium" && riskLevel != "high" && riskLevel != "critical" {
		return ""
	}
	signature := strings.TrimSpace(stringFromMap(final, "signature"))
	if signature == "" {
		return ""
	}
	grade := strings.TrimSpace(stringFromMap(final, "grade"))
	ruleVersion := strings.TrimSpace(stringFromMap(final, "rule_version"))
	verdict := strings.TrimSpace(stringFromMap(final, "verdict"))
	recommendation := strings.TrimSpace(stringFromMap(final, "recommendation"))
	claims, _ := userFromContext(r.Context())
	if target == "" {
		target = strings.TrimSpace(stringFromMap(envelope, "target"))
	}
	message := verdict
	if message == "" {
		message = "ARVIS produced a signed " + riskLevel + "-risk verdict."
	}
	id, err := alerts.Emit(r.Context(), h.DB, alerts.Event{
		AuthSubject: claims.Sub,
		Source:      "arvis",
		EventType:   alerts.EventARVISVerdictCreated,
		Severity:    riskLevel,
		Target:      target,
		Title:       "ARVIS signed verdict: " + strings.ToUpper(riskLevel),
		Message:     message,
		DedupeKey:   arvisAlertDedupeKey(claims.Sub, signature),
		EvidenceRef: signature,
		Payload:     arvisAlertPayload(target, grade, riskLevel, recommendation, signature, ruleVersion),
	})
	if err != nil {
		return ""
	}
	return id
}

// queueARVISCustomerTelegram is best-effort. Telegram/provider failure must not
// change the deterministic ARVIS response returned to the customer.
func (h *Handler) queueARVISCustomerTelegram(r *http.Request, target string, envelope map[string]any) bool {
	if r == nil || h == nil || h.DB == nil {
		return false
	}
	claims, ok := userFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Sub) == "" {
		return false
	}
	message, resultID := arvisCustomerTelegramMessage(target, envelope)
	if message == "" {
		return false
	}
	queued, err := cryptobrief.New(h.DB).QueueARVISResult(r.Context(), claims.Sub, resultID, message)
	return err == nil && queued
}

func arvisCustomerTelegramMessage(target string, envelope map[string]any) (string, string) {
	if envelope == nil {
		return "", ""
	}
	if target == "" {
		target = strings.TrimSpace(stringFromMap(envelope, "target"))
	}
	status := strings.TrimSpace(stringFromMap(envelope, "status"))
	if status == "" {
		status = "completed"
	}
	final, _ := envelope["final_verdict"].(map[string]any)
	riskLevel := strings.TrimSpace(stringFromMap(final, "risk_level"))
	grade := strings.TrimSpace(stringFromMap(final, "grade"))
	verdict := strings.TrimSpace(stringFromMap(final, "verdict"))
	recommendation := strings.TrimSpace(stringFromMap(final, "recommendation"))
	signature := strings.TrimSpace(stringFromMap(final, "signature"))
	hasEvidence, _ := envelope["has_live_evidence"].(bool)

	parts := []string{"🛡️ Koschei Web3 · ARVIS Scan Result"}
	if target != "" {
		parts = append(parts, "Target: "+target)
	}
	parts = append(parts, "Status: "+strings.ToUpper(status))
	if riskLevel != "" {
		parts = append(parts, "Risk: "+strings.ToUpper(riskLevel))
	}
	if grade != "" {
		parts = append(parts, "Grade: "+grade)
	}
	if verdict != "" {
		parts = append(parts, "Verdict: "+verdict)
	}
	if recommendation != "" {
		parts = append(parts, "Recommendation: "+recommendation)
	}
	if hasEvidence {
		parts = append(parts, "Evidence: live evidence verified")
	} else {
		parts = append(parts, "Evidence: incomplete / pending — not treated as SAFE")
	}
	parts = append(parts, arvisTelegramContextLines(envelope)...)
	if signature != "" {
		parts = append(parts, "Signature: "+signature)
	}
	return strings.Join(parts, "\n"), signature
}

func arvisTelegramContextLines(envelope map[string]any) []string {
	report, _ := envelope["investigation_report"].(map[string]any)
	if report == nil {
		return nil
	}
	out := []string{}
	trade, _ := report["trade_ledger_aggregates"].(map[string]any)
	if strings.EqualFold(strings.TrimSpace(stringFromMap(trade, "market_behavior_status")), "bounded_pattern_observed") {
		out = append(out, "Market behavior: suspicious bounded timing pattern observed — requires corroboration")
	}
	coordination, _ := report["actor_coordination_intelligence"].(map[string]any)
	if strings.EqualFold(strings.TrimSpace(stringFromMap(coordination, "status")), "coordination_patterns_observed") {
		out = append(out, "Actor coordination: evidence-backed correlation pattern(s) observed — not an identity or wrongdoing claim")
	}
	promotion, _ := report["public_promotion_intelligence"].(map[string]any)
	if strings.EqualFold(strings.TrimSpace(stringFromMap(promotion, "status")), "cross_asset_public_promotion_overlap_observed") {
		out = append(out, "Public promotion: cross-asset public-source overlap observed — context only")
	}
	return out
}

func arvisAlertDedupeKey(authSubject, signature string) string {
	scope := strings.TrimSpace(authSubject)
	if scope == "" {
		scope = "unscoped"
	}
	return "arvis-verdict:" + scope + ":" + strings.TrimSpace(signature)
}

func arvisAlertPayload(target, grade, riskLevel, recommendation, signature, ruleVersion string) map[string]any {
	return map[string]any{
		"target":         target,
		"grade":          grade,
		"risk_level":     riskLevel,
		"recommendation": recommendation,
		"signature":      signature,
		"rule_version":   ruleVersion,
	}
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}
