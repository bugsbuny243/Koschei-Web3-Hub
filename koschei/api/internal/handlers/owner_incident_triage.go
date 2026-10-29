package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

const ownerIncidentTriageSchema = "koschei.owner-incident-triage.v1"
const ownerIncidentTriageBoundary = "operator-assigned severity and incident status are triage metadata, not independent evidence of compromise, on-chain safety, or ARVIS verdict authority"

type ownerIncidentTriageItem struct {
	Incident                ownerIncidentRecord              `json:"incident"`
	AgeSeconds              int64                            `json:"age_seconds"`
	SinceLastUpdateSeconds  int64                            `json:"since_last_update_seconds"`
	TimeToResolutionSeconds *int64                           `json:"time_to_resolution_seconds,omitempty"`
	Active                  bool                             `json:"active"`
	AnalystAssignment       *ownerIncidentAssignment         `json:"analyst_assignment,omitempty"`
	Escalation              *ownerIncidentEscalation         `json:"escalation,omitempty"`
	OwnerReview             *ownerIncidentReviewProof        `json:"owner_review,omitempty"`
	WorkflowMilestones      *ownerIncidentWorkflowMilestones `json:"workflow_milestones,omitempty"`
}

func incidentElapsedSeconds(start, end time.Time) int64 {
	if start.IsZero() || end.Before(start) {
		return 0
	}
	return int64(end.Sub(start) / time.Second)
}

func buildOwnerIncidentTriage(items []ownerIncidentRecord, now time.Time) ([]ownerIncidentTriageItem, map[string]int, map[string]int) {
	out := make([]ownerIncidentTriageItem, 0, len(items))
	byStatus := make(map[string]int)
	bySeverity := make(map[string]int)
	for _, item := range items {
		byStatus[item.Status]++
		bySeverity[item.Severity]++
		entry := ownerIncidentTriageItem{
			Incident:               item,
			AgeSeconds:             incidentElapsedSeconds(item.CreatedAt, now),
			SinceLastUpdateSeconds: incidentElapsedSeconds(item.UpdatedAt, now),
			Active:                 item.Status == "open" || item.Status == "investigating" || item.Status == "contained",
		}
		if item.ResolvedAt != nil && !item.ResolvedAt.Before(item.CreatedAt) {
			elapsed := incidentElapsedSeconds(item.CreatedAt, *item.ResolvedAt)
			entry.TimeToResolutionSeconds = &elapsed
		}
		out = append(out, entry)
	}
	return out, byStatus, bySeverity
}

// ownerIncidentTriage is an opt-in owner-only, read-only projection of the
// existing incident registry. It does not define SLA deadlines, infer analyst
// assignment, generate incidents or mutate verdict/alert/evidence records.
func (h *Handler) ownerIncidentTriage(w http.ResponseWriter, r *http.Request) {
	if h == nil || (h.DBRead == nil && h.DB == nil) {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident triage store is unavailable")
		return
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validOwnerIncidentStatus(status) {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident status")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident triage limit")
			return
		}
		limit = parsed
	}
	if limit > 250 {
		limit = 250
	}
	rows, err := db.QueryContext(r.Context(), ownerIncidentSelectSQL()+
		" WHERE ($1='' OR status=$1) ORDER BY updated_at DESC LIMIT $2", status, limit)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident triage is unavailable")
		return
	}
	defer rows.Close()
	incidents := make([]ownerIncidentRecord, 0)
	for rows.Next() {
		item, err := scanOwnerIncident(rows)
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident triage could not be decoded")
			return
		}
		incidents = append(incidents, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident triage is unavailable")
		return
	}
	if err := rows.Close(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident triage rows could not be closed")
		return
	}
	asOf := time.Now().UTC()
	items, statusCounts, severityCounts := buildOwnerIncidentTriage(incidents, asOf)
	assignments, err := latestOwnerIncidentAssignments(r.Context(), db, incidents)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident analyst assignments are unavailable")
		return
	}
	applyOwnerIncidentAssignments(items, assignments)
	escalations, err := latestOwnerIncidentEscalations(r.Context(), db, incidents)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident escalation ledger is unavailable")
		return
	}
	applyOwnerIncidentEscalations(items, escalations)
	reviews, err := ownerIncidentReviewProofs(r.Context(), db, incidents)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Owner manual-review audit is unavailable")
		return
	}
	applyOwnerIncidentReviewProofs(items, reviews)
	milestones, err := earliestOwnerIncidentMilestones(r.Context(), db, incidents)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident workflow milestones are unavailable")
		return
	}
	applyOwnerIncidentMilestones(items, milestones)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                 true,
		"schema_version":     ownerIncidentTriageSchema,
		"decision_boundary":  ownerIncidentTriageBoundary,
		"as_of":              asOf,
		"scope":              "bounded_latest_updated_incidents_not_complete_history",
		"sla_state":          "not_configured_no_deadline_or_authenticated_assignment_claim",
		"count":              len(items),
		"counts_by_status":   statusCounts,
		"counts_by_severity": severityCounts,
		"items":              items,
	})
}
