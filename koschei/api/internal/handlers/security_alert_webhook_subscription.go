package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strings"

	"koschei/api/internal/alerts"
)

type securityAlertWebhookSubscriptionRequest struct {
	EndpointID string   `json:"endpoint_id"`
	Enabled    *bool    `json:"enabled,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
}

var supportedSecurityAlertEvents = map[string]bool{
	alerts.EventSecurityAlertCreated:      true,
	alerts.EventARVISVerdictCreated:       true,
	alerts.EventTransactionGuardDecision: true,
}

func (h *Handler) SecurityAlertWebhookSubscription(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database_unavailable"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		endpointID := strings.TrimSpace(r.URL.Query().Get("endpoint_id"))
		h.writeSecurityAlertWebhookSubscription(w, r, claims.Sub, endpointID)
	case http.MethodPost:
		var input securityAlertWebhookSubscriptionRequest
		if err := decodeJSON(r, &input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return
		}
		input.EndpointID = strings.TrimSpace(input.EndpointID)
		if !webhookUUIDPattern.MatchString(input.EndpointID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_webhook_id"})
			return
		}
		events, err := normalizeSecurityAlertEvents(input.EventTypes)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		current, err := loadWebhookEventTypes(r, h.DB, input.EndpointID, claims.Sub)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook_not_found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		updated := mutateWebhookEvents(current, events, enabled)
		res, err := h.DB.ExecContext(r.Context(), `
			UPDATE webhook_endpoints SET event_types=$1,updated_at=now()
			WHERE id=$2 AND auth_subject=$3`, stringSliceToPGArray(updated), input.EndpointID, claims.Sub)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		count, _ := res.RowsAffected()
		if count == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "endpoint_id": input.EndpointID, "enabled": enabled, "event_types": updated, "security_event_types": selectedSecurityAlertEvents(updated)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) writeSecurityAlertWebhookSubscription(w http.ResponseWriter, r *http.Request, authSubject, endpointID string) {
	if !webhookUUIDPattern.MatchString(endpointID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_webhook_id"})
		return
	}
	current, err := loadWebhookEventTypes(r, h.DB, endpointID, authSubject)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook_not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "endpoint_id": endpointID, "event_types": current, "security_event_types": selectedSecurityAlertEvents(current), "supported_security_event_types": sortedSupportedSecurityAlertEvents()})
}

func loadWebhookEventTypes(r *http.Request, db *sql.DB, endpointID, authSubject string) ([]string, error) {
	var raw string
	err := db.QueryRowContext(r.Context(), `SELECT event_types::text FROM webhook_endpoints WHERE id=$1 AND auth_subject=$2`, endpointID, authSubject).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return parsePGTextArray(raw), nil
}

func normalizeSecurityAlertEvents(input []string) ([]string, error) {
	if len(input) == 0 {
		return []string{alerts.EventSecurityAlertCreated}, nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range input {
		value := strings.ToLower(strings.TrimSpace(raw))
		if !supportedSecurityAlertEvents[value] {
			return nil, errors.New("unsupported_security_event_type")
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out, nil
}

func mutateWebhookEvents(current, selected []string, enabled bool) []string {
	set := map[string]bool{}
	for _, eventType := range current {
		eventType = strings.TrimSpace(eventType)
		if eventType != "" {
			set[eventType] = true
		}
	}
	for _, eventType := range selected {
		if enabled {
			set[eventType] = true
		} else {
			delete(set, eventType)
		}
	}
	out := make([]string, 0, len(set))
	for eventType := range set {
		out = append(out, eventType)
	}
	sort.Strings(out)
	return out
}

func selectedSecurityAlertEvents(current []string) []string {
	out := []string{}
	for _, eventType := range current {
		if supportedSecurityAlertEvents[eventType] {
			out = append(out, eventType)
		}
	}
	sort.Strings(out)
	return out
}

func sortedSupportedSecurityAlertEvents() []string {
	out := make([]string, 0, len(supportedSecurityAlertEvents))
	for eventType := range supportedSecurityAlertEvents {
		out = append(out, eventType)
	}
	sort.Strings(out)
	return out
}
