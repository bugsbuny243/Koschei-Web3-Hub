package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// An analyst ID is an owner-managed operational label, not an authenticated
// account, a permission grant, or proof of an analyst having acknowledged work.
type ownerIncidentAssignment struct {
	Status    string    `json:"status"`
	AnalystID string    `json:"analyst_id,omitempty"`
	Tier      string    `json:"tier,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

const ownerIncidentAssignmentPayloadKey = "soc_assignment_v1"

func validIncidentAnalystTier(tier string) bool {
	return tier == "L1" || tier == "L2" || tier == "L3"
}

func validIncidentAnalystID(id string) bool {
	if len(id) < 3 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		alphanumeric := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if i == 0 && !alphanumeric {
			return false
		}
		if !alphanumeric && c != '_' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

// Existing migration 123 constrains action_type to known values. A structured,
// versioned "note" is backwards compatible and preserves the full audit trail.
// No schema constraint is dropped or broadened.
func ownerIncidentAssignmentEvent(action, analystID, tier string) (string, map[string]any) {
	if action == "unassign_analyst" {
		return "note", map[string]any{
			ownerIncidentAssignmentPayloadKey: map[string]any{
				"status": "unassigned",
			},
		}
	}
	return "note", map[string]any{
		ownerIncidentAssignmentPayloadKey: map[string]any{
			"status": "assigned",
			"analyst_id": analystID,
			"tier": tier,
		},
	}
}

func latestOwnerIncidentAssignments(ctx context.Context, db *sql.DB, incidents []ownerIncidentRecord) (map[string]ownerIncidentAssignment, error) {
	assignments := make(map[string]ownerIncidentAssignment)
	if len(incidents) == 0 {
		return assignments, nil
	}
	ids := make([]string, 0, len(incidents))
	for _, item := range incidents {
		ids = append(ids, item.ID)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT ON (incident_id)
			incident_id::text, payload, created_at
		FROM security_incident_actions
		WHERE incident_id = ANY($1::uuid[])
		  AND action_type = 'note'
		  AND payload ? 'soc_assignment_v1'
		ORDER BY incident_id, created_at DESC, id DESC
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var payloadBytes []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &payloadBytes, &createdAt); err != nil {
			return nil, err
		}
		var envelope struct {
			Assignment struct {
				Status    string `json:"status"`
				Tier      string `json:"tier"`
				AnalystID string `json:"analyst_id"`
			} `json:"soc_assignment_v1"`
		}
		if err := json.Unmarshal(payloadBytes, &envelope); err != nil {
			return nil, err
		}
		switch envelope.Assignment.Status {
		case "unassigned":
			assignments[id] = ownerIncidentAssignment{Status: "unassigned", UpdatedAt: createdAt}
		case "assigned":
			if !validIncidentAnalystTier(envelope.Assignment.Tier) || !validIncidentAnalystID(envelope.Assignment.AnalystID) {
				return nil, fmt.Errorf("invalid persisted analyst assignment for incident %s", id)
			}
			assignments[id] = ownerIncidentAssignment{
				Status: "assigned", Tier: envelope.Assignment.Tier,
				AnalystID: envelope.Assignment.AnalystID, UpdatedAt: createdAt,
			}
		default:
			return nil, fmt.Errorf("unknown persisted analyst assignment status for incident %s", id)
		}
	}
	return assignments, rows.Err()
}

func applyOwnerIncidentAssignments(items []ownerIncidentTriageItem, assignments map[string]ownerIncidentAssignment) {
	for i := range items {
		item := &items[i]
		if snapshot, ok := assignments[strings.TrimSpace(item.Incident.ID)]; ok {
			item.AnalystAssignment = &snapshot
		}
	}
}
