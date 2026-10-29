package handlers

import (
	"context"
	"net/http"
	"time"
)

const ownerSOCReadinessVersion = "koschei.soc-runtime-readiness.v1"

// No source identifiers, credentials, DSNs, customer data or case contents are
// returned by this owner-only diagnostic. Values are bounded to a read sample.
type ownerSOCReadinessSnapshot struct {
	SchemaVersion        string    `json:"schema_version"`
	AsOf                 time.Time `json:"as_of"`
	State                string    `json:"state"`
	ReadSource           string    `json:"read_source"`
	CasesTableReadable   bool      `json:"cases_table_readable"`
	ActionsTableReadable bool      `json:"actions_table_readable"`
	SampledCases         int       `json:"sampled_cases"`
	AnyAuditedActions    bool      `json:"any_audited_actions"`
	AssignedCaseSamples  int       `json:"assigned_case_samples"`
	EscalationSamples    int       `json:"escalation_samples"`
	Scope                string    `json:"scope"`
	Boundary             string    `json:"boundary"`
}

const socReadinessScope = "read_only_latest_50_cases; audit_event_existence; latest_assignment_and_escalation_projections_only"
const socReadinessBoundary = "DB reads verify only this owner API's storage read path; no write acceptance, data completeness, active coverage, analyst identity, SLA compliance, ARVIS verdict safety or deployed enforcement is implied"

func socReadinessState(sampleCount int, anyAction bool) string {
	if sampleCount == 0 {
		return "not_accepted_no_case_observations"
	}
	if !anyAction {
		return "not_accepted_no_audit_event_observations"
	}
	return "read_path_observed_not_end_to_end_accepted"
}

func socReadinessUnavailable(w http.ResponseWriter, snapshot ownerSOCReadinessSnapshot) {
	snapshot.State = "unavailable_not_accepted"
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "readiness": snapshot})
}

// OwnerSOCReadiness is exposed *only* under the pre-existing ownerOnly route.
// It cannot attest to actual SOC staffing, chain monitoring or production SLA.
func (h *Handler) OwnerSOCReadiness(w http.ResponseWriter, r *http.Request) {
	snapshot := ownerSOCReadinessSnapshot{
		SchemaVersion: ownerSOCReadinessVersion,
		AsOf:          time.Now().UTC(),
		State:         "unavailable_not_accepted",
		Scope:         socReadinessScope,
		Boundary:      socReadinessBoundary,
	}
	if h == nil || (h.DBRead == nil && h.DB == nil) {
		socReadinessUnavailable(w, snapshot)
		return
	}
	db := h.DBRead
	snapshot.ReadSource = "configured_read_store"
	if db == nil {
		db = h.DB
		snapshot.ReadSource = "primary_db_fallback"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		socReadinessUnavailable(w, snapshot)
		return
	}

	// A missing table is a true failure: do not report empty as healthy.
	var casesPresent, actionsPresent bool
	err := db.QueryRowContext(ctx, `
		SELECT to_regclass('security_incident_cases') IS NOT NULL,
		       to_regclass('security_incident_actions') IS NOT NULL
	`).Scan(&casesPresent, &actionsPresent)
	if err != nil || !casesPresent || !actionsPresent {
		socReadinessUnavailable(w, snapshot)
		return
	}

	// Probe both read paths even if the underlying tables contain zero rows.
	var caseExists, auditExists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM security_incident_cases LIMIT 1),
		       EXISTS (SELECT 1 FROM security_incident_actions LIMIT 1)
	`).Scan(&caseExists, &auditExists)
	if err != nil {
		socReadinessUnavailable(w, snapshot)
		return
	}
	snapshot.CasesTableReadable = true
	snapshot.ActionsTableReadable = true
	snapshot.AnyAuditedActions = auditExists

	rows, err := db.QueryContext(ctx, `
		SELECT id::text FROM security_incident_cases
		ORDER BY updated_at DESC, id DESC LIMIT 50
	`)
	if err != nil {
		socReadinessUnavailable(w, snapshot)
		return
	}
	sampled := make([]ownerIncidentRecord, 0, 50)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			socReadinessUnavailable(w, snapshot)
			return
		}
		sampled = append(sampled, ownerIncidentRecord{ID: id})
	}
	readErr := rows.Err()
	closeErr := rows.Close()
	if readErr != nil || closeErr != nil {
		socReadinessUnavailable(w, snapshot)
		return
	}
	snapshot.SampledCases = len(sampled)
	// The table-read result must agree with the sampled-query observation.
	if caseExists && len(sampled) == 0 || !caseExists && len(sampled) > 0 {
		socReadinessUnavailable(w, snapshot)
		return
	}
	assignments, err := latestOwnerIncidentAssignments(ctx, db, sampled)
	if err != nil {
		socReadinessUnavailable(w, snapshot)
		return
	}
	escalations, err := latestOwnerIncidentEscalations(ctx, db, sampled)
	if err != nil {
		socReadinessUnavailable(w, snapshot)
		return
	}
	for _, entry := range assignments {
		if entry.Status == "assigned" {
			snapshot.AssignedCaseSamples++
		}
	}
	snapshot.EscalationSamples = len(escalations)
	snapshot.State = socReadinessState(snapshot.SampledCases, snapshot.AnyAuditedActions)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "readiness": snapshot})
}

