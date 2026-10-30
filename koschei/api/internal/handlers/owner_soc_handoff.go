package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"koschei/api/internal/socops"
)

const ownerIncidentHandoffSchemaV2 = "koschei.soc-incident-handoff.v2"

var errSOCHandoffLimit = errors.New("SOC handoff exceeds export limits")
var errSOCHandoffHistory = errors.New("SOC handoff history is inconsistent")

type socHandoffEvent struct {
	ID        int64           `json:"id"`
	Version   int64           `json:"version"`
	Action    string          `json:"action"`
	Actor     string          `json:"actor"`
	Summary   string          `json:"summary"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
	CreatedAt time.Time       `json:"created_at"`
}

type socHandoffOperations struct {
	Version         string            `json:"version"`
	ReadSource      string            `json:"read_source"`
	AsOf            time.Time         `json:"as_of"`
	State           socops.State      `json:"state"`
	InitialSeverity string            `json:"initial_severity"`
	SLA             socops.SLA        `json:"sla"`
	Events          []socHandoffEvent `json:"events"`
}

func sameSOCState(a, b socops.State) bool {
	if a.Tier != b.Tier || a.Assignee != b.Assignee || a.Version != b.Version {
		return false
	}
	if a.AcknowledgedAt == nil || b.AcknowledgedAt == nil {
		return a.AcknowledgedAt == nil && b.AcknowledgedAt == nil
	}
	return a.AcknowledgedAt.Equal(*b.AcknowledgedAt)
}

// Validate the entire bounded revision chain, not just its last row. This is a
// consistency check of operator records, not independent evidence verification.
func validateSOCHandoffHistory(state socops.State, events []socHandoffEvent) error {
	previous := socops.State{Tier: "L1"}
	for _, event := range events {
		var before, after socops.State
		if json.Unmarshal(event.Before, &before) != nil || json.Unmarshal(event.After, &after) != nil ||
			!sameSOCState(previous, before) || event.Version != previous.Version+1 || after.Version != event.Version {
			return errSOCHandoffHistory
		}
		command := socops.Command{Action: event.Action, Summary: event.Summary, ExpectedVersion: &before.Version}
		if event.Action == "assign" {
			command.Assignee = after.Assignee
		}
		if event.Action == "escalate" {
			command.Tier = after.Tier
		}
		expected, err := socops.Apply(before, command, "open", event.CreatedAt)
		if err != nil || !sameSOCState(expected, after) {
			return errSOCHandoffHistory
		}
		previous = after
	}
	if !sameSOCState(previous, state) {
		return errSOCHandoffHistory
	}
	return nil
}

func readSOCHandoff(ctx context.Context, tx *sql.Tx, id string, now time.Time) (*socHandoffOperations, error) {
	item, err := scanSOCItem(tx.QueryRowContext(ctx, socQueueSelect+` WHERE c.id=$1::uuid`, id), now)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,version,action,actor,summary,before_state,after_state,created_at
 FROM security_incident_soc_events WHERE incident_id=$1::uuid ORDER BY version ASC LIMIT $2`, id, ownerIncidentHandoffActionLimit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]socHandoffEvent, 0)
	totalBytes := 0
	for rows.Next() {
		var event socHandoffEvent
		if err := rows.Scan(&event.ID, &event.Version, &event.Action, &event.Actor, &event.Summary, &event.Before, &event.After, &event.CreatedAt); err != nil {
			return nil, err
		}
		totalBytes += len(event.Before) + len(event.After) + len(event.Actor) + len(event.Summary)
		if len(event.Before)+len(event.After) > ownerIncidentHandoffMaxPayloadBytes || totalBytes > ownerIncidentHandoffMaxActionsBytes || len(events) >= ownerIncidentHandoffActionLimit {
			return nil, errSOCHandoffLimit
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := validateSOCHandoffHistory(item.Operations, events); err != nil {
		return nil, err
	}
	return &socHandoffOperations{Version: socops.Version, ReadSource: "primary_db", AsOf: now, State: item.Operations, InitialSeverity: item.InitialSeverity, SLA: item.SLA, Events: events}, nil
}
