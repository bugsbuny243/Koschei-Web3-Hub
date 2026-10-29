package handlers

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// These milestones report owner workflow events, not independently verified
// analyst responses or on-chain containment and never establish SLA compliance.
type ownerIncidentWorkflowMilestones struct {
	FirstInvestigationAt        *time.Time `json:"first_investigation_at,omitempty"`
	FirstContainmentRecordAt    *time.Time `json:"first_containment_record_at,omitempty"`
	FirstResolutionRecordAt     *time.Time `json:"first_resolution_record_at,omitempty"`
	FirstReopenAt               *time.Time `json:"first_reopen_at,omitempty"`
	SecondsToFirstInvestigation *int64     `json:"seconds_to_first_investigation,omitempty"`
	SecondsToFirstContainment   *int64     `json:"seconds_to_first_containment_record,omitempty"`
	SecondsToFirstResolution    *int64     `json:"seconds_to_first_resolution_record,omitempty"`
	EvidenceBasis               string     `json:"evidence_basis"`
}

type ownerIncidentMilestoneTimes struct {
	Investigation *time.Time
	Containment   *time.Time
	Resolution    *time.Time
	Reopen        *time.Time
}

func incidentRecordedElapsed(createdAt time.Time, timestamp *time.Time) (*time.Time, *int64) {
	if createdAt.IsZero() || timestamp == nil || timestamp.IsZero() || timestamp.Before(createdAt) {
		return nil, nil
	}
	value := timestamp.UTC()
	seconds := int64(value.Sub(createdAt) / time.Second)
	return &value, &seconds
}

func buildOwnerIncidentWorkflowMilestones(createdAt time.Time, timestamps ownerIncidentMilestoneTimes) *ownerIncidentWorkflowMilestones {
	investigate, investigateSeconds := incidentRecordedElapsed(createdAt, timestamps.Investigation)
	contain, containSeconds := incidentRecordedElapsed(createdAt, timestamps.Containment)
	resolve, resolveSeconds := incidentRecordedElapsed(createdAt, timestamps.Resolution)
	reopen, _ := incidentRecordedElapsed(createdAt, timestamps.Reopen)
	if investigate == nil && contain == nil && resolve == nil && reopen == nil {
		return nil
	}
	return &ownerIncidentWorkflowMilestones{
		FirstInvestigationAt:        investigate,
		FirstContainmentRecordAt:    contain,
		FirstResolutionRecordAt:     resolve,
		FirstReopenAt:               reopen,
		SecondsToFirstInvestigation: investigateSeconds,
		SecondsToFirstContainment:   containSeconds,
		SecondsToFirstResolution:    resolveSeconds,
		EvidenceBasis:               "owner_incident_actions_timestamp_not_verified_response_or_enforcement",
	}
}

func earliestOwnerIncidentMilestones(ctx context.Context, db *sql.DB, incidents []ownerIncidentRecord) (map[string]*ownerIncidentWorkflowMilestones, error) {
	byIncident := make(map[string]*ownerIncidentWorkflowMilestones)
	if len(incidents) == 0 {
		return byIncident, nil
	}
	ids := make([]string, 0, len(incidents))
	created := make(map[string]time.Time, len(incidents))
	for _, item := range incidents {
		ids = append(ids, item.ID)
		created[item.ID] = item.CreatedAt
	}
	// One bounded aggregate per returned case, not all-time action export.
	// MIN is historical: reopened cases retain their first recorded milestones.
	rows, err := db.QueryContext(ctx, `
		SELECT incident_id::text,
			MIN(created_at) FILTER (WHERE action_type = 'investigate'),
			MIN(created_at) FILTER (WHERE action_type = 'contain'),
			MIN(created_at) FILTER (WHERE action_type = 'resolve'),
			MIN(created_at) FILTER (WHERE action_type = 'reopen')
		FROM security_incident_actions
		WHERE incident_id = ANY($1::uuid[])
			AND action_type IN ('investigate','contain','resolve','reopen')
		GROUP BY incident_id
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var investigation, containment, resolution, reopen sql.NullTime
		if err := rows.Scan(&id, &investigation, &containment, &resolution, &reopen); err != nil {
			return nil, err
		}
		times := ownerIncidentMilestoneTimes{}
		if investigation.Valid {
			times.Investigation = &investigation.Time
		}
		if containment.Valid {
			times.Containment = &containment.Time
		}
		if resolution.Valid {
			times.Resolution = &resolution.Time
		}
		if reopen.Valid {
			times.Reopen = &reopen.Time
		}
		if milestone := buildOwnerIncidentWorkflowMilestones(created[id], times); milestone != nil {
			byIncident[id] = milestone
		}
	}
	return byIncident, rows.Err()
}

func applyOwnerIncidentMilestones(items []ownerIncidentTriageItem, milestones map[string]*ownerIncidentWorkflowMilestones) {
	for i := range items {
		if item, exists := milestones[items[i].Incident.ID]; exists {
			items[i].WorkflowMilestones = item
		}
	}
}
