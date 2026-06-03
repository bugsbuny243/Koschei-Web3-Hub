package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type analyticsEventInput struct {
	EventName string         `json:"event_name"`
	Email     string         `json:"email"`
	Path      string         `json:"path"`
	Metadata  map[string]any `json:"metadata"`
}

type analyticsEventRecord struct {
	ID        string         `json:"id"`
	EventName string         `json:"event_name"`
	Email     *string        `json:"email"`
	Path      string         `json:"path"`
	Referrer  string         `json:"referrer"`
	UserAgent string         `json:"user_agent"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

func (h *Handler) AnalyticsEvent(w http.ResponseWriter, r *http.Request) {
	var req analyticsEventInput
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	eventName := strings.TrimSpace(req.EventName)
	if eventName == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if claims, ok := verifiedClaimsFromBearer(r); ok {
		email = strings.ToLower(strings.TrimSpace(claims.Email))
	}
	var emailValue any
	if email != "" {
		emailValue = email
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	if h.DB != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()
		_, _ = h.DB.ExecContext(ctx, `
			INSERT INTO analytics_events (event_name, email, path, referrer, user_agent, metadata)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
			eventName,
			emailValue,
			strings.TrimSpace(req.Path),
			strings.TrimSpace(r.Header.Get("Referer")),
			strings.TrimSpace(r.Header.Get("User-Agent")),
			string(metadataJSON),
		)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) AdminAnalyticsEvents(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}

	counts := map[string]int64{
		"events_today":              0,
		"signup_success":            0,
		"login_success":             0,
		"metadata_generate_success": 0,
		"risk_scan_success":         0,
	}
	var eventsToday int64
	if err := h.DB.QueryRowContext(r.Context(), `
		SELECT count(*)
		FROM analytics_events
		WHERE created_at >= CURRENT_DATE`).Scan(&eventsToday); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	counts["events_today"] = eventsToday

	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT event_name, count(*)
		FROM analytics_events
		WHERE event_name IN ('signup_success', 'login_success', 'metadata_generate_success', 'risk_scan_success')
		GROUP BY event_name`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var eventName string
		var count int64
		if err := rows.Scan(&eventName, &count); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		counts[eventName] = count
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}

	eventRows, err := h.DB.QueryContext(r.Context(), `
		SELECT id::text, event_name, email, COALESCE(path, ''), COALESCE(referrer, ''), COALESCE(user_agent, ''), COALESCE(metadata, '{}'::jsonb), created_at
		FROM analytics_events
		ORDER BY created_at DESC
		LIMIT 100`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer eventRows.Close()

	events := make([]analyticsEventRecord, 0)
	for eventRows.Next() {
		var event analyticsEventRecord
		var metadataRaw []byte
		if err := eventRows.Scan(&event.ID, &event.EventName, &event.Email, &event.Path, &event.Referrer, &event.UserAgent, &metadataRaw, &event.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		if err := json.Unmarshal(metadataRaw, &event.Metadata); err != nil {
			event.Metadata = map[string]any{}
		}
		events = append(events, event)
	}
	if err := eventRows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "counts": counts, "events": events})
}

func verifiedClaimsFromBearer(r *http.Request) (neonJWTClaims, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return neonJWTClaims{}, false
	}
	claims, err := parseAndVerifyNeonJWT(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")))
	if err != nil {
		return neonJWTClaims{}, false
	}
	return claims, true
}
