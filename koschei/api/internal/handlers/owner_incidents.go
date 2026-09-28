package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const ownerIncidentDecisionBoundary = "operator incident state links evidence and response actions; it never changes ARVIS evidence, grades, signatures, or deterministic verdict authority"

type ownerIncidentRecord struct {
	ID          string     `json:"id"`
	IncidentRef string     `json:"incident_ref"`
	Title       string     `json:"title"`
	Severity    string     `json:"severity"`
	Status      string     `json:"status"`
	Network     string     `json:"network"`
	Target      string     `json:"target"`
	Summary     string     `json:"summary"`
	EvidenceRefs []string  `json:"evidence_refs"`
	AlertRefs    []string  `json:"alert_refs"`
	DossierRefs  []string  `json:"dossier_refs"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ResolvedAt *time.Time  `json:"resolved_at,omitempty"`
}

type ownerIncidentActionRecord struct {
	ID         string         `json:"id"`
	ActionType string         `json:"action_type"`
	Actor      string         `json:"actor"`
	Summary    string         `json:"summary"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  time.Time      `json:"created_at"`
}

type ownerIncidentCreateRequest struct {
	Title        string   `json:"title"`
	Severity     string   `json:"severity"`
	Network      string   `json:"network"`
	Target       string   `json:"target"`
	Summary      string   `json:"summary"`
	EvidenceRefs []string `json:"evidence_refs"`
	AlertRefs    []string `json:"alert_refs"`
	DossierRefs  []string `json:"dossier_refs"`
}

type ownerIncidentActionRequest struct {
	Action   string `json:"action"`
	Summary  string `json:"summary"`
	Severity string `json:"severity"`
	Ref      string `json:"ref"`
}

type incidentScanner interface {
	Scan(dest ...any) error
}

func validOwnerIncidentSeverity(value string) bool {
	switch value {
	case "info", "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

func validOwnerIncidentStatus(value string) bool {
	switch value {
	case "open", "investigating", "contained", "resolved", "closed":
		return true
	default:
		return false
	}
}

func trimIncidentRefs(in []string, max int) []string {
	if max <= 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, raw := range in {
		value := strings.TrimSpace(raw)
		if value == "" || len(value) > 512 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= max {
			break
		}
	}
	return out
}

func scanOwnerIncident(scanner incidentScanner) (ownerIncidentRecord, error) {
	var item ownerIncidentRecord
	var evidenceRaw, alertsRaw, dossiersRaw []byte
	var resolved sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.IncidentRef, &item.Title, &item.Severity, &item.Status,
		&item.Network, &item.Target, &item.Summary,
		&evidenceRaw, &alertsRaw, &dossiersRaw,
		&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &resolved,
	)
	if err != nil {
		return item, err
	}
	item.EvidenceRefs = []string{}
	item.AlertRefs = []string{}
	item.DossierRefs = []string{}
	_ = json.Unmarshal(evidenceRaw, &item.EvidenceRefs)
	_ = json.Unmarshal(alertsRaw, &item.AlertRefs)
	_ = json.Unmarshal(dossiersRaw, &item.DossierRefs)
	if resolved.Valid {
		value := resolved.Time
		item.ResolvedAt = &value
	}
	return item, nil
}

func ownerIncidentSelectSQL() string {
	return `SELECT id::text, incident_ref, title, severity, status, network, target, summary,
	       evidence_refs, alert_refs, dossier_refs, created_by, created_at, updated_at, resolved_at
	FROM security_incident_cases`
}

func (h *Handler) OwnerIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ownerIncidentList(w, r)
	case http.MethodPost:
		h.ownerIncidentCreate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) ownerIncidentList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident store is unavailable")
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validOwnerIncidentStatus(status) {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident status")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 250 {
		limit = 250
	}

	rows, err := h.DBRead.QueryContext(r.Context(), ownerIncidentSelectSQL()+`
		WHERE ($1='' OR status=$1)
		ORDER BY updated_at DESC
		LIMIT $2`, status, limit)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident list is unavailable")
		return
	}
	defer rows.Close()

	items := make([]ownerIncidentRecord, 0)
	for rows.Next() {
		item, err := scanOwnerIncident(rows)
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident list could not be decoded")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident list is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "incidents": items, "decision_boundary": ownerIncidentDecisionBoundary,
	})
}

func (h *Handler) ownerIncidentCreate(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident store is unavailable")
		return
	}
	var input ownerIncidentCreateRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident request")
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Severity = strings.ToLower(strings.TrimSpace(input.Severity))
	input.Network = strings.TrimSpace(input.Network)
	input.Target = strings.TrimSpace(input.Target)
	input.Summary = strings.TrimSpace(input.Summary)
	if input.Severity == "" {
		input.Severity = "medium"
	}
	if input.Title == "" || len(input.Title) > 180 || len(input.Summary) > 4000 || len(input.Network) > 96 || len(input.Target) > 512 {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Incident fields are invalid")
		return
	}
	if !validOwnerIncidentSeverity(input.Severity) {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident severity")
		return
	}
	input.EvidenceRefs = trimIncidentRefs(input.EvidenceRefs, 64)
	input.AlertRefs = trimIncidentRefs(input.AlertRefs, 64)
	input.DossierRefs = trimIncidentRefs(input.DossierRefs, 64)
	evidenceJSON, _ := json.Marshal(input.EvidenceRefs)
	alertJSON, _ := json.Marshal(input.AlertRefs)
	dossierJSON, _ := json.Marshal(input.DossierRefs)

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident transaction is unavailable")
		return
	}
	defer tx.Rollback()

	var created ownerIncidentRecord
	row := tx.QueryRowContext(r.Context(), `
		INSERT INTO security_incident_cases
			(title,severity,status,network,target,summary,evidence_refs,alert_refs,dossier_refs,created_by)
		VALUES ($1,$2,'open',$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,'owner')
		RETURNING id::text, incident_ref, title, severity, status, network, target, summary,
		          evidence_refs, alert_refs, dossier_refs, created_by, created_at, updated_at, resolved_at
	`, input.Title, input.Severity, input.Network, input.Target, input.Summary, string(evidenceJSON), string(alertJSON), string(dossierJSON))
	created, err = scanOwnerIncident(row)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident could not be created")
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"severity": input.Severity, "network": input.Network, "target": input.Target,
		"evidence_refs": input.EvidenceRefs, "alert_refs": input.AlertRefs, "dossier_refs": input.DossierRefs,
	})
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO security_incident_actions (incident_id,action_type,actor,summary,payload)
		VALUES ($1::uuid,'created','owner',$2,$3::jsonb)
	`, created.ID, input.Summary, string(payload)); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident audit action could not be recorded")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident transaction could not be committed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok": true, "incident": created, "decision_boundary": ownerIncidentDecisionBoundary,
	})
}

func (h *Handler) OwnerIncidentItem(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident store is unavailable")
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/owner/incidents/"))
	if id == "" || strings.Contains(id, "/") || len(id) > 64 {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident id")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.ownerIncidentGet(w, r, id)
	case http.MethodPost:
		h.ownerIncidentAction(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) ownerIncidentGet(w http.ResponseWriter, r *http.Request, id string) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	item, err := scanOwnerIncident(db.QueryRowContext(r.Context(), ownerIncidentSelectSQL()+" WHERE id=$1::uuid", id))
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, APICodeNotFound, "Incident was not found")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident is unavailable")
		return
	}
	rows, err := db.QueryContext(r.Context(), `
		SELECT id::text, action_type, actor, summary, payload, created_at
		FROM security_incident_actions
		WHERE incident_id=$1::uuid
		ORDER BY created_at ASC, id ASC
		LIMIT 500`, id)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident action timeline is unavailable")
		return
	}
	defer rows.Close()
	actions := make([]ownerIncidentActionRecord, 0)
	for rows.Next() {
		var action ownerIncidentActionRecord
		var payloadRaw []byte
		if err := rows.Scan(&action.ID, &action.ActionType, &action.Actor, &action.Summary, &payloadRaw, &action.CreatedAt); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident action timeline could not be decoded")
			return
		}
		action.Payload = map[string]any{}
		_ = json.Unmarshal(payloadRaw, &action.Payload)
		actions = append(actions, action)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "incident": item, "actions": actions, "decision_boundary": ownerIncidentDecisionBoundary,
	})
}

func incidentActionStatus(action string) string {
	switch action {
	case "investigate":
		return "investigating"
	case "contain":
		return "contained"
	case "resolve":
		return "resolved"
	case "reopen":
		return "open"
	case "close":
		return "closed"
	default:
		return ""
	}
}

func (h *Handler) ownerIncidentAction(w http.ResponseWriter, r *http.Request, id string) {
	var input ownerIncidentActionRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident action")
		return
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	input.Summary = strings.TrimSpace(input.Summary)
	input.Severity = strings.ToLower(strings.TrimSpace(input.Severity))
	input.Ref = strings.TrimSpace(input.Ref)
	if len(input.Summary) > 4000 || len(input.Ref) > 512 {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Incident action fields are invalid")
		return
	}
	validAction := map[string]bool{
		"note": true, "investigate": true, "contain": true, "resolve": true, "reopen": true, "close": true,
		"severity_change": true, "link_evidence": true, "link_alert": true, "link_dossier": true,
	}
	if !validAction[input.Action] {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Unsupported incident action")
		return
	}
	if input.Action == "severity_change" && !validOwnerIncidentSeverity(input.Severity) {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid incident severity")
		return
	}
	if strings.HasPrefix(input.Action, "link_") && input.Ref == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Incident reference is required")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident transaction is unavailable")
		return
	}
	defer tx.Rollback()

	var exists string
	if err := tx.QueryRowContext(r.Context(), "SELECT id::text FROM security_incident_cases WHERE id=$1::uuid FOR UPDATE", id).Scan(&exists); err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, APICodeNotFound, "Incident was not found")
		return
	} else if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident could not be locked")
		return
	}

	if status := incidentActionStatus(input.Action); status != "" {
		_, err = tx.ExecContext(r.Context(), `
			UPDATE security_incident_cases
			SET status=$2,
			    resolved_at=CASE
			        WHEN $2='resolved' THEN now()
			        WHEN $2 IN ('open','investigating','contained') THEN NULL
			        ELSE resolved_at
			    END,
			    updated_at=now()
			WHERE id=$1::uuid`, id, status)
	} else if input.Action == "severity_change" {
		_, err = tx.ExecContext(r.Context(), "UPDATE security_incident_cases SET severity=$2,updated_at=now() WHERE id=$1::uuid", id, input.Severity)
	} else if input.Action == "link_evidence" {
		_, err = tx.ExecContext(r.Context(), `UPDATE security_incident_cases
			SET evidence_refs=CASE WHEN evidence_refs ? $2 THEN evidence_refs ELSE evidence_refs || jsonb_build_array($2::text) END, updated_at=now()
			WHERE id=$1::uuid`, id, input.Ref)
	} else if input.Action == "link_alert" {
		_, err = tx.ExecContext(r.Context(), `UPDATE security_incident_cases
			SET alert_refs=CASE WHEN alert_refs ? $2 THEN alert_refs ELSE alert_refs || jsonb_build_array($2::text) END, updated_at=now()
			WHERE id=$1::uuid`, id, input.Ref)
	} else if input.Action == "link_dossier" {
		_, err = tx.ExecContext(r.Context(), `UPDATE security_incident_cases
			SET dossier_refs=CASE WHEN dossier_refs ? $2 THEN dossier_refs ELSE dossier_refs || jsonb_build_array($2::text) END, updated_at=now()
			WHERE id=$1::uuid`, id, input.Ref)
	} else {
		_, err = tx.ExecContext(r.Context(), "UPDATE security_incident_cases SET updated_at=now() WHERE id=$1::uuid", id)
	}
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident action could not be applied")
		return
	}

	payload, _ := json.Marshal(map[string]any{"severity": input.Severity, "ref": input.Ref})
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO security_incident_actions (incident_id,action_type,actor,summary,payload)
		VALUES ($1::uuid,$2,'owner',$3,$4::jsonb)
	`, id, input.Action, input.Summary, string(payload)); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident action audit record could not be written")
		return
	}
	item, err := scanOwnerIncident(tx.QueryRowContext(r.Context(), ownerIncidentSelectSQL()+" WHERE id=$1::uuid", id))
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident could not be reloaded")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident action could not be committed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "incident": item, "decision_boundary": ownerIncidentDecisionBoundary,
	})
}
