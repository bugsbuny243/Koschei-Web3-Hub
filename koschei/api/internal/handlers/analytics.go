package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var (
	errInvalidAnalyticsEvent    = errors.New("invalid_event")
	errInvalidAnalyticsMetadata = errors.New("invalid_metadata")
)

var allowedAnalyticsEvents = map[string]bool{
	"landing_view":       true,
	"hub_view":           true,
	"register_page_view": true,
	"register_submit":    true,
	"signup_success":     true,
	"login_success":      true,
	"metadata_generate":  true,
	"risk_scan":          true,
	"chains_refresh":     true,
}

type analyticsEventRequest struct {
	EventName string          `json:"event_name"`
	Email     string          `json:"email"`
	Path      string          `json:"path"`
	Referrer  string          `json:"referrer"`
	UserAgent string          `json:"user_agent"`
	Metadata  json.RawMessage `json:"metadata"`
}

type normalizedAnalyticsEvent struct {
	EventName string
	Email     sql.NullString
	Path      string
	Referrer  string
	UserAgent string
	Metadata  []byte
}

func (h *Handler) AnalyticsEvent(w http.ResponseWriter, r *http.Request) {
	var req analyticsEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	event, err := normalizeAnalyticsEvent(req, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if _, err := h.DB.ExecContext(r.Context(), `
		INSERT INTO analytics_events (event_name, email, path, referrer, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		event.EventName, event.Email, event.Path, event.Referrer, event.UserAgent, string(event.Metadata)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func normalizeAnalyticsEvent(req analyticsEventRequest, r *http.Request) (normalizedAnalyticsEvent, error) {
	eventName := strings.ToLower(strings.TrimSpace(req.EventName))
	if !allowedAnalyticsEvents[eventName] {
		return normalizedAnalyticsEvent{}, errInvalidAnalyticsEvent
	}

	metadata := []byte(`{}`)
	if len(req.Metadata) > 0 && strings.TrimSpace(string(req.Metadata)) != "" {
		if !json.Valid(req.Metadata) {
			return normalizedAnalyticsEvent{}, errInvalidAnalyticsMetadata
		}
		metadata = req.Metadata
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		path = strings.TrimSpace(r.Header.Get("X-Koschei-Page"))
	}
	if path == "" {
		path = "/"
	}

	referrer := strings.TrimSpace(req.Referrer)
	if referrer == "" {
		referrer = strings.TrimSpace(r.Referer())
	}

	userAgent := strings.TrimSpace(req.UserAgent)
	if userAgent == "" {
		userAgent = strings.TrimSpace(r.UserAgent())
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	return normalizedAnalyticsEvent{
		EventName: eventName,
		Email:     sql.NullString{String: email, Valid: email != ""},
		Path:      limitAnalyticsField(path),
		Referrer:  limitAnalyticsField(referrer),
		UserAgent: limitAnalyticsField(userAgent),
		Metadata:  metadata,
	}, nil
}

func limitAnalyticsField(value string) string {
	const maxLen = 2048
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
