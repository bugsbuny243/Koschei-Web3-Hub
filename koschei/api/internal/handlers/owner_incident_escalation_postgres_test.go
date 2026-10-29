package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Uses migration 123 on a dedicated PostgreSQL test database.
// If the environment variable is absent, the test is skipped (not accepted).
func TestOwnerIncidentEscalationCyclePostgres(t *testing.T) {
	dsn := socPostgresAcceptanceDSN(t)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	title := fmt.Sprintf("soc-escalation-acceptance-%d", time.Now().UnixNano())
	var id string
	if err := db.QueryRow(`
		INSERT INTO security_incident_cases (title, severity, status, summary)
		VALUES ($1,'high','open','test fixture only') RETURNING id::text
	`, title).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Cleanup is restricted to this run's known, uniquely titled fixture.
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})

	h := &Handler{DB: db, DBRead: db}
	action := func(body string, expected int) {
		t.Helper()
		rec := httptest.NewRecorder()
		h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodPost,
			"/api/owner/incidents/"+id, strings.NewReader(body)))
		if rec.Code != expected {
			t.Fatalf("status %d expected %d for %s: %s", rec.Code, expected, body, rec.Body.String())
		}
	}
	action(`{"action":"request_escalation","tier":"L2","summary":"Needs manual analysis"}`, http.StatusConflict)
	action(`{"action":"assign_analyst","analyst_id":"soc-first","tier":"L1"}`, http.StatusOK)
	action(`{"action":"request_escalation","tier":"L1","summary":"Needs manual analysis"}`, http.StatusBadRequest)
	action(`{"action":"request_escalation","tier":"L2","summary":"Needs manual analysis"}`, http.StatusOK)

	items := []ownerIncidentRecord{{ID: id, Status: "open"}}
	escalations, err := latestOwnerIncidentEscalations(context.Background(), db, items)
	if err != nil {
		t.Fatal(err)
	}
	if escalations[id].State != "requested" || escalations[id].TargetTier != "L2" {
		t.Fatalf("request not persisted: %#v", escalations[id])
	}
	action(`{"action":"assign_analyst","analyst_id":"soc-second","tier":"L2"}`, http.StatusOK)
	assigned, err := latestOwnerIncidentAssignments(context.Background(), db, items)
	if err != nil {
		t.Fatal(err)
	}
	projection, _, _ := buildOwnerIncidentTriage(items, time.Now())
	applyOwnerIncidentAssignments(projection, assigned)
	applyOwnerIncidentEscalations(projection, escalations)
	if projection[0].Escalation == nil ||
		projection[0].Escalation.State != "superseded_by_later_assignment_not_acknowledged" {
		t.Fatalf("new assignment did not mark request as superseded: %#v", projection)
	}
	action(`{"action":"clear_escalation","summary":"Manual review no longer required"}`, http.StatusOK)
	action(`{"action":"clear_escalation","summary":"Repeated clear must fail"}`, http.StatusConflict)
	escalations, err = latestOwnerIncidentEscalations(context.Background(), db, items)
	if err != nil {
		t.Fatal(err)
	}
	if escalations[id].State != "cleared" || escalations[id].TargetTier != "" {
		t.Fatalf("clear not persisted: %#v", escalations[id])
	}

	rec := httptest.NewRecorder()
	h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodGet, "/api/owner/incidents/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("audit timeline returned %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Actions []ownerIncidentActionRecord `json:"actions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	// assignment, requested, reassigned, cleared. Failed attempts create no rows.
	if len(response.Actions) != 4 {
		t.Fatalf("unexpected audit action count %d, expected 4", len(response.Actions))
	}
}
