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

func redactMetadata(metadata map[string]any) map[string]any {
	redacted := map[string]any{}
	for key, value := range metadata {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "authorization") || strings.Contains(lower, "jwt") {
			redacted[key] = "[redacted]"
			continue
		}
		redacted[key] = value
	}
	return redacted
}
