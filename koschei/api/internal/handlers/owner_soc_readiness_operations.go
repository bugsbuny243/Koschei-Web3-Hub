package handlers

import (
	"context"
	"database/sql"
)

// Separate from the legacy read-store projection: the operations API uses the
// primary database, even when DBRead is configured to a different database.
type socOperationsReadiness struct {
	State            string `json:"state"`
	ReadSource       string `json:"read_source"`
	MetadataReadable bool   `json:"metadata_readable"`
	HistoryReadable  bool   `json:"history_readable"`
	Boundary         string `json:"boundary"`
}

func probeSOCOperations(ctx context.Context, db *sql.DB) socOperationsReadiness {
	out := socOperationsReadiness{State: "unavailable_not_accepted", ReadSource: "primary_db",
		Boundary: "schema read access only; does not prove writes, immutable audit enforcement, staffing, SLA compliance or end-to-end readiness"}
	if db == nil {
		return out
	}
	// LIMIT 0 validates the actual required columns and SELECT permissions
	// without returning any identifiers, assignments or event payloads.
	queries := []string{
		`SELECT incident_id,tier,assignee,acknowledged_at,version,updated_at FROM security_incident_soc LIMIT 0`,
		`SELECT id,incident_id,version,action,actor,summary,before_state,after_state,created_at FROM security_incident_soc_events LIMIT 0`,
	}
	for i, q := range queries {
		rows, err := db.QueryContext(ctx, q)
		if err != nil {
			return out
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil || closeErr != nil {
			return out
		}
		if i == 0 {
			out.MetadataReadable = true
		} else {
			out.HistoryReadable = true
		}
	}
	out.State = "schema_read_observed_not_write_accepted"
	return out
}
