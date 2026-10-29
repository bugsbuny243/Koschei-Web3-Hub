package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// An escalation is an owner-authored *request for human review*. It does not
// authenticate a target analyst, dispatch a notification or satisfy an SLA.
const ownerIncidentEscalationPayloadKey = "soc_escalation_v1"

type ownerIncidentEscalation struct {
	State       string    `json:"state"`
	TargetTier  string    `json:"target_tier,omitempty"`
	RequestedAt time.Time `json:"event_at"`
}

func incidentAnalystTierRank(tier string) int {
	switch tier {
	case "L1":
		return 1
	case "L2":
		return 2
	case "L3":
		return 3
	default:
		return 0
	}
}

func ownerIncidentEscalationEvent(action, tier string) (string, map[string]any) {
	state := "requested"
	if action == "clear_escalation" {
		state = "cleared"
	}
	event := map[string]any{"state": state}
	if state == "requested" {
		event["target_tier"] = tier
	}
	// Migration 123 explicitly permits "note"; do not broaden the constraint.
	return "note", map[string]any{ownerIncidentEscalationPayloadKey: event}
}

func latestOwnerIncidentAssignmentTier(ctx context.Context, tx *sql.Tx, incidentID string) (string, error) {
	var state, tier string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(payload->'soc_assignment_v1'->>'status',''),
			   COALESCE(payload->'soc_assignment_v1'->>'tier','')
		FROM security_incident_actions
		WHERE incident_id=$1::uuid AND action_type='note'
		  AND payload ? 'soc_assignment_v1'
		ORDER BY created_at DESC, id DESC LIMIT 1
	`, incidentID).Scan(&state, &tier)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if state == "unassigned" {
		return "", nil
	}
	if state != "assigned" || incidentAnalystTierRank(tier) == 0 {
		return "", fmt.Errorf("invalid assignment audit event")
	}
	return tier, nil
}

func latestOwnerIncidentEscalationState(ctx context.Context, tx *sql.Tx, incidentID string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(payload->'soc_escalation_v1'->>'state','')
		FROM security_incident_actions
		WHERE incident_id=$1::uuid AND action_type='note'
		  AND payload ? 'soc_escalation_v1'
		ORDER BY created_at DESC, id DESC LIMIT 1
	`, incidentID).Scan(&state)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if state != "requested" && state != "cleared" {
		return "", fmt.Errorf("invalid escalation audit state")
	}
	return state, nil
}

func latestOwnerIncidentEscalations(ctx context.Context, db *sql.DB, incidents []ownerIncidentRecord) (map[string]ownerIncidentEscalation, error) {
	out := make(map[string]ownerIncidentEscalation)
	if len(incidents) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(incidents))
	for _, item := range incidents {
		ids = append(ids, item.ID)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT ON (incident_id) incident_id::text, payload, created_at
		FROM security_incident_actions
		WHERE incident_id=ANY($1::uuid[]) AND action_type='note'
		  AND payload ? 'soc_escalation_v1'
		ORDER BY incident_id, created_at DESC, id DESC
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw []byte
		var stamped time.Time
		if err := rows.Scan(&id, &raw, &stamped); err != nil {
			return nil, err
		}
		var envelope struct {
			Event struct {
				State      string `json:"state"`
				TargetTier string `json:"target_tier"`
			} `json:"soc_escalation_v1"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		v := ownerIncidentEscalation{State: envelope.Event.State, RequestedAt: stamped}
		switch v.State {
		case "requested":
			if envelope.Event.TargetTier != "L2" && envelope.Event.TargetTier != "L3" {
				return nil, fmt.Errorf("invalid persisted escalation tier for incident %s", id)
			}
			v.TargetTier = envelope.Event.TargetTier
		case "cleared":
			if envelope.Event.TargetTier != "" {
				return nil, fmt.Errorf("invalid cleared escalation for incident %s", id)
			}
		default:
			return nil, fmt.Errorf("unknown persisted escalation state for incident %s", id)
		}
		out[id] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func applyOwnerIncidentEscalations(items []ownerIncidentTriageItem, events map[string]ownerIncidentEscalation) {
	for i := range items {
		item := &items[i]
		event, exists := events[item.Incident.ID]
		if !exists {
			continue
		}
		if event.State == "requested" {
			if !item.Active {
				event.State = "historical_request_case_inactive"
			} else if item.AnalystAssignment != nil &&
				item.AnalystAssignment.UpdatedAt.After(event.RequestedAt) {
				event.State = "superseded_by_later_assignment_not_acknowledged"
			}
		}
		item.Escalation = &event
	}
}
