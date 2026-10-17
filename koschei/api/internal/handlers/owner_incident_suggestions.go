package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const ownerIncidentSuggestionSchema = "koschei.owner-incident-review-candidates.v1"
const ownerIncidentSuggestionBoundary = "persisted alert severity is a triage signal, not verified compromise, malicious intent, chain safety, or incident status; only an owner may explicitly create or link an incident"

type ownerIncidentAlertSource struct {
	ID              string
	Source          string
	EventType       string
	Severity        string
	Target          string
	Title           string
	OccurrenceCount int
	CreatedAt       time.Time
	LastSeenAt      time.Time
	StoredPayload   []byte
}

type ownerIncidentReviewCandidate struct {
	AlertID               string                     `json:"alert_id"`
	Source                string                     `json:"source"`
	EventType             string                     `json:"event_type"`
	Severity              string                     `json:"severity"`
	Target                string                     `json:"target"`
	AlertTitle            string                     `json:"alert_title"`
	OccurrenceCount       int                        `json:"occurrence_count"`
	CreatedAt             time.Time                  `json:"created_at"`
	LastSeenAt            time.Time                  `json:"last_seen_at"`
	StoredPayloadSHA256   string                     `json:"stored_payload_sha256"`
	PayloadHashScope      string                     `json:"payload_hash_scope"`
	ChainEvidenceVerified bool                       `json:"chain_evidence_verified"`
	EvidenceState         string                     `json:"evidence_state"`
	ReviewState           string                     `json:"review_state"`
	SuggestedIncident     ownerIncidentCreateRequest `json:"suggested_incident"`
}

func ownerIncidentReviewCandidateFromAlert(alert ownerIncidentAlertSource) ownerIncidentReviewCandidate {
	// The existing incident-create handler resolves this alert ID against the
	// durable source record again, inside its own transaction. An unverified
	// title, target or severity must never grant incident/response authority.
	title := strings.TrimSpace(alert.Title)
	if title == "" {
		title = "Persisted security alert"
	}
	runes := []rune("Review security alert: " + title)
	if len(runes) > 180 {
		runes = runes[:180]
	}
	target := strings.TrimSpace(alert.Target)
	if len(target) > 512 {
		target = target[:512]
	}
	severity := strings.ToLower(strings.TrimSpace(alert.Severity))
	if !validOwnerIncidentSeverity(severity) {
		severity = "medium"
	}
	payloadHash := sha256.Sum256(alert.StoredPayload)
	return ownerIncidentReviewCandidate{
		AlertID: alert.ID, Source: alert.Source, EventType: alert.EventType,
		Severity: severity, Target: target, AlertTitle: title,
		OccurrenceCount: alert.OccurrenceCount, CreatedAt: alert.CreatedAt, LastSeenAt: alert.LastSeenAt,
		StoredPayloadSHA256:   hex.EncodeToString(payloadHash[:]),
		PayloadHashScope:      "sha256_of_postgresql_jsonb_text_read_snapshot_not_source_signature",
		ChainEvidenceVerified: false,
		EvidenceState:         "persisted_alert_not_independent_evidence_verification",
		ReviewState:           "owner_review_required",
		SuggestedIncident:     ownerIncidentCreateRequest{
			Title: string(runes), Severity: severity, Target: target,
			// security_alert_events has no canonical network column.
			// Do not infer a chain from the alert's target or payload.
			Network:      "",
			Summary:      "Manual triage proposal from a persisted security alert. Verify the underlying source and scope before opening or linking an incident. This is not proof of compromise.",
			AlertRefs:    []string{alert.ID},
			EvidenceRefs: []string{},
			DossierRefs:  []string{},
		},
	}
}

// ownerIncidentSuggestions is an opt-in, read-only projection of the existing
// owner-only GET /api/owner/incidents route. Default incident listing, incident
// writes, alert delivery, ARVIS verdicts and audit timelines are not modified.
func (h *Handler) ownerIncidentSuggestions(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident review store is unavailable")
		return
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident review store is unavailable")
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident review limit")
			return
		}
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if len(target) > 512 {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident review target")
		return
	}

	// The indexed created_at bound avoids unbounded scans. A deduplicated alert
	// first created before this window will not appear here, even when recently
	// repeated. This is an explicit triage limitation, never a clean bill.
	rows, err := db.QueryContext(r.Context(), `
		SELECT a.id::text, a.source, a.event_type, a.severity, a.target,
		       a.title, a.occurrence_count, a.created_at, a.last_seen_at,
		       a.payload::text
		FROM security_alert_events a
		WHERE a.created_at >= now() - interval '7 days'
		  AND a.severity IN ('high', 'critical')
		  AND ($2='' OR a.target=$2)
		  AND NOT EXISTS (
		    SELECT 1 FROM security_incident_cases c
		    WHERE c.status IN ('open','investigating','contained')
		      AND c.alert_refs ? a.id::text
		  )
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $1
	`, limit, target)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident review candidates are unavailable")
		return
	}
	defer rows.Close()

	suggestions := make([]ownerIncidentReviewCandidate, 0)
	for rows.Next() {
		var alert ownerIncidentAlertSource
		if err := rows.Scan(
			&alert.ID, &alert.Source, &alert.EventType, &alert.Severity,
			&alert.Target, &alert.Title, &alert.OccurrenceCount,
			&alert.CreatedAt, &alert.LastSeenAt, &alert.StoredPayload,
		); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident review candidate could not be decoded")
			return
		}
		suggestions = append(suggestions, ownerIncidentReviewCandidateFromAlert(alert))
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident review candidates are unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"schema_version":    ownerIncidentSuggestionSchema,
		"decision_boundary": ownerIncidentSuggestionBoundary,
		"source":            "persisted_security_alert_events",
		"window":            "alerts_created_in_last_7_days",
		"limitations": []string{
			"Severity is inherited from alert metadata and is not independent evidence verification.",
			"Older deduplicated alerts are outside the seven-day created_at window even if last_seen_at is recent.",
			"Only alerts without a linked active incident are presented; no incident is created or changed.",
			"Alert payloads are not interpreted as canonical network or actor attribution.",
			"Stored JSONB SHA-256 is a snapshot fingerprint only, not an on-chain signature, independent verification, or immutable history.",
		},
		"suggestions": suggestions,
	})
}
