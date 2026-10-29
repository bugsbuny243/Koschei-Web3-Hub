package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/socops"
)

var socIncidentID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type socQueueItem struct {
	ID              string       `json:"id"`
	IncidentRef     string       `json:"incident_ref"`
	Title           string       `json:"title"`
	Severity        string       `json:"severity"`
	InitialSeverity string       `json:"initial_severity"`
	Status          string       `json:"status"`
	Network         string       `json:"network"`
	CreatedAt       time.Time    `json:"created_at"`
	Operations      socops.State `json:"operations"`
	SLA             socops.SLA   `json:"sla"`
}

// No query in this module modifies source incidents, evidence or verdicts.
// Primary reads avoid presenting stale revisions immediately after an action.
const socQueueSelect = `SELECT c.id::text,c.incident_ref,c.title,c.severity,c.status,c.network,c.created_at,
 COALESCE(s.tier,'L1'),COALESCE(s.assignee,''),s.acknowledged_at,COALESCE(s.version,0),
 COALESCE(initial.payload->>'severity',''),
 LEAST(s.acknowledged_at,response.responded_at)
 FROM security_incident_cases c
 LEFT JOIN security_incident_soc s ON s.incident_id=c.id
 LEFT JOIN LATERAL (
   SELECT payload FROM security_incident_actions WHERE incident_id=c.id AND action_type='created'
   ORDER BY created_at,id LIMIT 1
 ) initial ON true
 LEFT JOIN LATERAL (
   SELECT MIN(created_at) AS responded_at FROM security_incident_actions
   WHERE incident_id=c.id AND action_type IN ('investigate','contain','resolve','close')
 ) response ON true`

func scanSOCItem(row incidentScanner, now time.Time) (socQueueItem, error) {
	var item socQueueItem
	var acknowledged, responded sql.NullTime
	err := row.Scan(&item.ID, &item.IncidentRef, &item.Title, &item.Severity, &item.Status, &item.Network, &item.CreatedAt,
		&item.Operations.Tier, &item.Operations.Assignee, &acknowledged, &item.Operations.Version, &item.InitialSeverity, &responded)
	if err != nil {
		return item, err
	}
	if acknowledged.Valid {
		item.Operations.AcknowledgedAt = &acknowledged.Time
	}
	var response *time.Time
	if responded.Valid {
		response = &responded.Time
	}
	item.SLA = socops.FirstResponse(item.InitialSeverity, item.Status, item.CreatedAt, now, response)
	return item, nil
}

func (h *Handler) OwnerSOCQueue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.DB == nil {
		socUnavailable(w)
		return
	}
	q := r.URL.Query()
	tier, status, severity, assignee := q.Get("tier"), q.Get("status"), q.Get("severity"), strings.TrimSpace(q.Get("assignee"))
	if status == "" {
		status = "active"
	}
	if (tier != "" && !socops.ValidTier(tier)) || (status != "active" && status != "all" && !validOwnerIncidentStatus(status)) || (severity != "" && !validOwnerIncidentSeverity(severity)) || len(assignee) > 120 || (q.Get("unassigned") != "" && q.Get("unassigned") != "1") {
		writeAPIError(w, 400, APICodeInvalidInput, "Invalid queue filter")
		return
	}
	limit, offset := 100, 0
	for name, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
		if raw := q.Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 || (name == "limit" && (n < 1 || n > 250)) || (name == "offset" && n > 100000) {
				writeAPIError(w, 400, APICodeInvalidInput, "Invalid pagination")
				return
			}
			*dest = n
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC()
	rows, err := h.DB.QueryContext(ctx, socQueueSelect+`
 WHERE ($1='' OR COALESCE(s.tier,'L1')=$1)
 AND ($2='all' OR ($2='active' AND c.status IN ('open','investigating','contained')) OR c.status=$2)
 AND ($3='' OR c.severity=$3) AND ($4='' OR COALESCE(s.assignee,'')=$4)
 AND (NOT $5 OR COALESCE(s.assignee,'')='')
 ORDER BY CASE c.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,c.created_at,c.id
 LIMIT $6 OFFSET $7`, tier, status, severity, assignee, q.Get("unassigned") == "1", limit+1, offset)
	if err != nil {
		socUnavailable(w)
		return
	}
	defer rows.Close()
	items := make([]socQueueItem, 0)
	for rows.Next() {
		item, err := scanSOCItem(rows, now)
		if err != nil {
			socUnavailable(w)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		socUnavailable(w)
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	counts := map[string]int{"pending": 0, "overdue": 0, "met": 0, "breached": 0, "unknown": 0}
	unassigned := 0
	for _, item := range items {
		counts[item.SLA.State]++
		if item.Operations.Assignee == "" {
			unassigned++
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "version": socops.Version, "incidents": items, "as_of": now, "has_more": hasMore, "offset": offset, "limit": limit,
		"telemetry": map[string]any{"scope": "returned_page", "cases": len(items), "unassigned": unassigned, "first_response": counts}, "decision_boundary": ownerIncidentDecisionBoundary})
}

func socUnavailable(w http.ResponseWriter) {
	writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC store is unavailable; no healthy or empty state can be inferred")
}

func (h *Handler) OwnerSOCCase(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(405)
		return
	}
	if h == nil || h.DB == nil {
		socUnavailable(w)
		return
	}
	id := r.URL.Query().Get("id")
	if !socIncidentID.MatchString(id) {
		writeAPIError(w, 400, APICodeInvalidInput, "Invalid incident id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if r.Method == http.MethodPost {
		h.ownerSOCAction(w, r, id)
		return
	}
	// One read-only snapshot keeps case revision and event timeline consistent.
	tx, err := h.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		socUnavailable(w)
		return
	}
	defer tx.Rollback()
	item, err := scanSOCItem(tx.QueryRowContext(ctx, socQueueSelect+` WHERE c.id=$1::uuid`, id), time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 404, APICodeNotFound, "Incident not found")
		return
	}
	if err != nil {
		socUnavailable(w)
		return
	}
	before := int64(0)
	if raw := r.URL.Query().Get("before_event"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before <= 0 {
			writeAPIError(w, 400, APICodeInvalidInput, "Invalid event cursor")
			return
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,version,action,actor,summary,before_state,after_state,created_at
 FROM security_incident_soc_events WHERE incident_id=$1::uuid AND ($2::bigint=0 OR id<$2)
 ORDER BY id DESC LIMIT 101`, id, before)
	if err != nil {
		socUnavailable(w)
		return
	}
	type event struct {
		ID        int64           `json:"id"`
		Version   int64           `json:"version"`
		Action    string          `json:"action"`
		Actor     string          `json:"actor"`
		Summary   string          `json:"summary"`
		Before    json.RawMessage `json:"before"`
		After     json.RawMessage `json:"after"`
		CreatedAt time.Time       `json:"created_at"`
	}
	events := make([]event, 0)
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.ID, &e.Version, &e.Action, &e.Actor, &e.Summary, &e.Before, &e.After, &e.CreatedAt); err != nil {
			rows.Close()
			socUnavailable(w)
			return
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		socUnavailable(w)
		return
	}
	more := len(events) > 100
	if more {
		events = events[:100]
	}
	if err := tx.Commit(); err != nil {
		socUnavailable(w)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "version": socops.Version, "incident": item, "events": events, "has_more_events": more, "decision_boundary": ownerIncidentDecisionBoundary})
}

func (h *Handler) ownerSOCAction(w http.ResponseWriter, r *http.Request, id string) {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeAPIError(w, 415, APICodeInvalidInput, "JSON content type required")
		return
	}
	var c socops.Command
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		writeAPIError(w, 400, APICodeInvalidInput, "Invalid SOC command")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || c.Validate() != nil {
		writeAPIError(w, 400, APICodeInvalidInput, "Invalid SOC command; revision and reason are required")
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		socUnavailable(w)
		return
	}
	defer tx.Rollback()
	var status string
	// Shares the legacy workflow's row lock, serializing close/reopen with SOC edits.
	err = tx.QueryRowContext(r.Context(), `SELECT status FROM security_incident_cases WHERE id=$1::uuid FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 404, APICodeNotFound, "Incident not found")
		return
	}
	if err != nil {
		socUnavailable(w)
		return
	}
	before := socops.State{Tier: "L1"}
	var ack sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT tier,assignee,acknowledged_at,version FROM security_incident_soc WHERE incident_id=$1::uuid`, id).Scan(&before.Tier, &before.Assignee, &ack, &before.Version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		socUnavailable(w)
		return
	}
	if ack.Valid {
		before.AcknowledgedAt = &ack.Time
	}
	var now time.Time
	if err := tx.QueryRowContext(r.Context(), `SELECT clock_timestamp()`).Scan(&now); err != nil {
		socUnavailable(w)
		return
	}
	after, err := socops.Apply(before, c, status, now)
	if err != nil {
		code := 400
		if errors.Is(err, socops.ErrConflict) || errors.Is(err, socops.ErrInactive) {
			code = 409
		}
		apiCode := APICodeInvalidInput
		if code == 409 {
			apiCode = APICodeConflict
		}
		writeAPIError(w, code, apiCode, err.Error())
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO security_incident_soc (incident_id,tier,assignee,acknowledged_at,version,updated_at)
 VALUES ($1::uuid,$2,$3,$4,$5,$6) ON CONFLICT (incident_id) DO UPDATE
 SET tier=EXCLUDED.tier,assignee=EXCLUDED.assignee,acknowledged_at=EXCLUDED.acknowledged_at,version=EXCLUDED.version,updated_at=EXCLUDED.updated_at`, id, after.Tier, after.Assignee, after.AcknowledgedAt, after.Version, now)
	if err != nil {
		socUnavailable(w)
		return
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO security_incident_soc_events (incident_id,version,action,actor,summary,before_state,after_state,created_at)
 VALUES ($1::uuid,$2,$3,'owner',$4,$5::jsonb,$6::jsonb,$7)`, id, after.Version, c.Action, c.Summary, string(beforeJSON), string(afterJSON), now)
	if err != nil {
		socUnavailable(w)
		return
	}
	if err := tx.Commit(); err != nil {
		socUnavailable(w)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "operations": after, "decision_boundary": ownerIncidentDecisionBoundary})
}
