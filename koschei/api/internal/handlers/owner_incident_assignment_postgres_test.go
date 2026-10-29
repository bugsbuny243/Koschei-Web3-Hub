package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Requires an isolated test database with migration 123 applied.
// Never point KOSCHEI_TEST_DATABASE_URL at a production database.
func TestOwnerIncidentAnalystAssignmentCyclePostgres17(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not configured; PostgreSQL acceptance is not verified")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	title := fmt.Sprintf("soc-assignment-integration-%d", time.Now().UnixNano())
	var id string
	err = db.QueryRow(`
		INSERT INTO security_incident_cases (title, severity, status, summary)
		VALUES ($1, 'medium', 'open', 'isolated test case')
		RETURNING id::text
	`, title).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Only remove the unique test-created fixture. Never touch other cases.
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})

	h := &Handler{DB: db, DBRead: db}
	post := func(body string) int {
		t.Helper()
		rec := httptest.NewRecorder()
		h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodPost,
			"/api/owner/incidents/"+id, strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("incident action failed status=%d body=%s", rec.Code, rec.Body.String())
		}
		return rec.Code
	}
	post(`{"action":"assign_analyst","analyst_id":"soc-first","tier":"L1"}`)
	post(`{"action":"assign_analyst","analyst_id":"soc-reviewer","tier":"L3"}`)

	snapshot, err := latestOwnerIncidentAssignments(
		httptest.NewRequest(http.MethodGet, "/", nil).Context(),
		db, []ownerIncidentRecord{{ID: id}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot[id].Status != "assigned" || snapshot[id].Tier != "L3" || snapshot[id].AnalystID != "soc-reviewer" {
		t.Fatalf("reassignment not reflected in latest audit view: %#v", snapshot[id])
	}

	post(`{"action":"unassign_analyst"}`)
	snapshot, err = latestOwnerIncidentAssignments(
		httptest.NewRequest(http.MethodGet, "/", nil).Context(),
		db, []ownerIncidentRecord{{ID: id}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot[id].Status != "unassigned" || snapshot[id].AnalystID != "" || snapshot[id].Tier != "" {
		t.Fatalf("unassignment not reflected in latest audit view: %#v", snapshot[id])
	}
	var count int
	if err := db.QueryRow(`
		SELECT count(*) FROM security_incident_actions
		WHERE incident_id=$1::uuid AND action_type='note' AND payload ? 'soc_assignment_v1'
	`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("audit history was lost or rewritten; want 3 events, got %d", count)
	}
	rec := httptest.NewRecorder()
	h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodGet, "/api/owner/incidents/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("owner incident timeline failed: %d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Actions []ownerIncidentActionRecord `json:"actions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 3 {
		t.Fatalf("original audit timeline did not preserve all assignments: %d", len(result.Actions))
	}
}
