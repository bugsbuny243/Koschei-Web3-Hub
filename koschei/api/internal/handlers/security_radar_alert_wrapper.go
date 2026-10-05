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
	"koschei/api/internal/sentinelclient"
)

const maxSecurityRadarAlertBody = 1 << 20

// SecurityRadarCheckWithAlerts preserves the existing investigation response
// contract and adds durable side effects only after the canonical customer scan
// completed. Security alerts remain severity-gated, customer Telegram delivery
// covers every successful scan, and Sentinel is observe-only commentary over a
// signed evidence-backed ARVIS result.
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
			alertID := h.emitARVISVerdictAlert(r, target, envelope)
			if alertID != "" {
				envelope["alert_event_id"] = alertID
			}
			if observation, ok := observeARVISWithSentinel(r, envelope); ok {
				envelope["sentinel_observation"] = observation
			}
			if h.queueARVISCustomerTelegram(r, target, envelope) {
				envelope["telegram_delivery_queued"] = true
			}
			if encoded, marshalErr := json.Marshal(envelope); marshalErr == nil {
				responseBody = encoded
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

func observeARVISWithSentinel(r *http.Request, envelope map[string]any) (sentinelclient.Observation, bool) {
	if r == nil {
		return sentinelclient.Observation{}, false
	}
	config := sentinelclient.ConfigFromEnv()
	if !config.Ready() {
		return sentinelclient.Observation{}, false
	}
	securityCase, ok := sentinelclient.CaseFromARVISEnvelope(envelope)
	if !ok {
		return sentinelclient.Observation{}, false
	}
	observation, err := sentinelclient.Observe(r.Context(), nil, config, securityCase)
	if err != nil {
		return sentinelclient.Observation{}, false
	}
	return observation, true
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
	riskLevel := arvisRiskLevelFromFinal(final)
	if riskLevel != "medium" && riskLevel != "high" && riskLevel != "critical" {
		return ""
	}
	signature := strings.TrimSpace(stringFromMap(final, "signature"))
	if signature == "" {
		return ""
	}
	grade := strings.TrimSpace(stringFromMap(final, "grade"))
	ruleVersion := strings.TrimSpace(firstNonEmptyString(stringFromMap(final, "rule_version"), stringFromMap(final, "ruleset_version")))
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

func arvisRiskLevelFromFinal(final map[string]any) string {
	if explicit := strings.ToLower(strings.TrimSpace(stringFromMap(final, "risk_level"))); explicit != "" {
		return explicit
	}
	switch strings.ToUpper(strings.TrimSpace(stringFromMap(final, "grade"))) {
	case "F":
		return "critical"
	case "D":
		return "high"
	case "C":
		return "medium"
	case "B", "A":
		return "low"
	default:
		return "info"
	}
}

// queueARVISCustomerTelegram is intentionally best-effort. A provider or queue
// problem must never change the deterministic scan result returned to the user.
// The cryptobrief subscription store already owns consent, pairing, recipient
// isolation, webhook verification, retries and provider delivery.
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
	riskLevel := arvisRiskLevelFromFinal(final)
	grade := strings.TrimSpace(stringFromMap(final, "grade"))
	verdict := strings.TrimSpace(stringFromMap(final, "verdict"))
	recommendation := strings.TrimSpace(stringFromMap(final, "recommendation"))
	signature := strings.TrimSpace(stringFromMap(final, "signature"))
	hasEvidence, _ := envelope["has_live_evidence"].(bool)

	parts := []string{"🛡️ Koschei ARVIS · Scan Result"}
	if target != "" {
		parts = append(parts, "Target: "+target)
	}
	parts = append(parts, "Status: "+strings.ToUpper(status))
	if riskLevel != "" && riskLevel != "info" {
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

	if summary, ok := envelope["analysis_summary"].(map[string]any); ok && len(summary) > 0 {
		if raw, err := json.Marshal(summary); err == nil {
			parts = append(parts, "Analysis: "+string(raw))
		}
	}
	if _, ok := envelope["sentinel_observation"]; ok {
		parts = append(parts, "Sentinel: observe-only commentary attached to the Web3 result")
	}
	if signature != "" {
		parts = append(parts, "Signature: "+signature)
	}
	parts = append(parts, "Koschei Professional · Web3 + ARVIS + Sentinel + Lang")
	return strings.Join(parts, "\n"), signature
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
