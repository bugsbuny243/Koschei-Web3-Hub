package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// The owner can record evidence of manually reviewing a case, not evidence of
// an analyst response, notification delivery, enforcement, or an SLA result.
const ownerIncidentReviewPayloadKey = "soc_owner_review_v1"
const ownerIncidentReviewEventName = "owner_review_recorded"

type ownerIncidentReviewProof struct {
	EvidenceState           string    `json:"evidence_state"`
	Actor                   string    `json:"actor"`
	FirstRecordedAt         time.Time `json:"first_recorded_at"`
	LatestRecordedAt        time.Time `json:"latest_recorded_at"`
	RecordCount             int64     `json:"record_count"`
	SecondsFromCaseCreation int64     `json:"seconds_from_case_creation"`
}

func ownerIncidentReviewEvent() (string, map[string]any) {
	// Migration 123 has an explicit allowlist. Existing note events can carry
	// our versioned namespace without any schema or legacy action changes.
	return "note", map[string]any{
		ownerIncidentReviewPayloadKey: map[string]any{
			"event": ownerIncidentReviewEventName,
			"scope": "manual_case_review_only",
		},
	}
}

// Query a bounded set of incident IDs. Every matching event must have a
// consistent owner-only payload; a corrupt audit ledger fails closed.
func ownerIncidentReviewProofs(ctx context.Context, db *sql.DB, incidents []ownerIncidentRecord) (map[string]ownerIncidentReviewProof, error) {
	proofs := make(map[string]ownerIncidentReviewProof)
	if len(incidents) == 0 {
		return proofs, nil
	}
	ids := make([]string, 0, len(incidents))
	createdAt := make(map[string]time.Time, len(incidents))
	for _, incident := range incidents {
		ids = append(ids, incident.ID)
		createdAt[incident.ID] = incident.CreatedAt
	}
	rows, err := db.QueryContext(ctx, `
		SELECT incident_id::text, MIN(created_at), MAX(created_at),
		       COUNT(*)::bigint,
		       COALESCE(bool_and(
		           actor='owner'
		           AND payload->'soc_owner_review_v1'->>'event'='owner_review_recorded'
		           AND payload->'soc_owner_review_v1'->>'scope'='manual_case_review_only'
		       ), false)
		FROM security_incident_actions
		WHERE incident_id=ANY($1::uuid[]) AND action_type='note'
		  AND payload ? 'soc_owner_review_v1'
		GROUP BY incident_id
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var first, last time.Time
		var count int64
		var valid bool
		if err := rows.Scan(&id, &first, &last, &count, &valid); err != nil {
			return nil, err
		}
		caseCreated, found := createdAt[id]
		if !found || !valid || count < 1 || caseCreated.IsZero() ||
			first.Before(caseCreated) || last.Before(first) {
			return nil, fmt.Errorf("owner review audit inconsistent for case %s", id)
		}
		proofs[id] = ownerIncidentReviewProof{
			EvidenceState:           "owner_manual_review_recorded",
			Actor:                   "owner",
			FirstRecordedAt:         first,
			LatestRecordedAt:        last,
			RecordCount:             count,
			SecondsFromCaseCreation: incidentElapsedSeconds(caseCreated, first),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return proofs, nil
}

func applyOwnerIncidentReviewProofs(items []ownerIncidentTriageItem, proofs map[string]ownerIncidentReviewProof) {
	for i := range items {
		if event, ok := proofs[items[i].Incident.ID]; ok {
			proof := event
			items[i].OwnerReview = &proof
		}
	}
}
