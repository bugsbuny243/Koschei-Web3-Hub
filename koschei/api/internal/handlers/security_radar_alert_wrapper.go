package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"koschei/api/internal/alerts"
)

// SecurityRadarCheckWithAlerts preserves the existing investigation response
// contract and adds a durable alert only after a signed, evidence-ready verdict
// has been produced. The alert pipeline never changes the deterministic grade.
func (h *Handler) SecurityRadarCheckWithAlerts(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body")
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

	alertID := ""
	if result.StatusCode >= 200 && result.StatusCode < 300 && h != nil && h.DB != nil {
		var envelope map[string]any
		if json.Unmarshal(responseBody, &envelope) == nil {
			alertID = h.emitARVISVerdictAlert(r, target, envelope)
			if alertID != "" {
				envelope["alert_event_id"] = alertID
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
	grade := strings.TrimSpace(stringFromMap(final, "grade"))
	verdict := strings.TrimSpace(stringFromMap(final, "verdict"))
	recommendation := strings.TrimSpace(stringFromMap(final, "recommendation"))
	riskIndex := numberFromMap(final, "risk_index")
	claims, _ := userFromContext(r.Context())
	if target == "" {
		target = strings.TrimSpace(stringFromMap(envelope, "target"))
	}
	dedupe := "arvis-verdict:" + signature
	if signature == "" {
		dedupe = "arvis-verdict:" + target + ":" + riskLevel + ":" + strconv.Itoa(riskIndex)
	}
	message := verdict
	if message == "" {
		message = "ARVIS produced a signed " + riskLevel + "-risk verdict."
	}
	id, err := alerts.Emit(r.Context(), h.DB, alerts.Event{
		AuthSubject: claims.Sub,
		Source: "arvis",
		EventType: alerts.EventARVISVerdictCreated,
		Severity: riskLevel,
		Target: target,
		Title: "ARVIS signed verdict: " + strings.ToUpper(riskLevel),
		Message: message,
		DedupeKey: dedupe,
		EvidenceRef: signature,
		Payload: map[string]any{
			"target": target,
			"grade": grade,
			"risk_index": riskIndex,
			"risk_level": riskLevel,
			"recommendation": recommendation,
			"signature": signature,
		},
	})
	if err != nil {
		return ""
	}
	return id
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func numberFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}
