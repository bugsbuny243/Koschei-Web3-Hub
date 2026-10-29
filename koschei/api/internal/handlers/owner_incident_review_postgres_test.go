package handlers

import (
	"context"
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

// Run only against an isolated PostgreSQL instance with migration 123 applied.
// The confirmation flag is mandatory to prevent accidental data writes to
// an arbitrary environment, including production.
func TestOwnerIncidentManualReviewPostgres(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("KOSCHEI_SOC_TEST_DB_CONFIRMATION") != "isolated" {
		t.Skip("isolated PostgreSQL + KOSCHEI_SOC_TEST_DB_CONFIRMATION=isolated required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	title := fmt.Sprintf("soc-owner-review-test-%d", time.Now().UnixNano())
	var id string
	if err := db.QueryRow(`
		INSERT INTO security_incident_cases (title,severity,status,summary)
		VALUES ($1,'high','open','isolated integration test fixture')
		RETURNING id::text
	`, title).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})
	h := &Handler{DB: db, DBRead: db}
	post := func(body string, want int) {
		t.Helper()
		rec := httptest.NewRecorder()
		h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodPost,
			"/api/owner/incidents/"+id, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("action returned HTTP %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	incidents := []ownerIncidentRecord{}
	item, err := scanOwnerIncident(db.QueryRow(ownerIncidentSelectSQL()+" WHERE id=$1::uuid", id))
	if err != nil {
		t.Fatal(err)
	}
	incidents = append(incidents, item)

	before, err := ownerIncidentReviewProofs(context.Background(), db, incidents)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("owner review record was fabricated: %#v", before)
	}
	post(`{"action":"record_owner_review","summary":"Owner inspected the linked incident references"}`, http.StatusOK)
	post(`{"action":"record_owner_review","summary":"Owner rechecked the case classification"}`, http.StatusOK)
	proofs, err := ownerIncidentReviewProofs(context.Background(), db, incidents)
	if err != nil {
		t.Fatal(err)
	}
	review, ok := proofs[id]
	if !ok || review.EvidenceState != "owner_manual_review_recorded" ||
		review.Actor != "owner" || review.RecordCount != 2 ||
		review.SecondsFromCaseCreation < 0 || review.LatestRecordedAt.Before(review.FirstRecordedAt) {
		t.Fatalf("manual review audit incorrect: %#v", review)
	}
	rec := httptest.NewRecorder()
	h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodGet, "/api/owner/incidents/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("incident audit timeline unavailable: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Actions []ownerIncidentActionRecord `json:"actions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Actions) != 2 {
		t.Fatalf("manual review audit history expected 2, got %d", len(body.Actions))
	}

	// Existing status guards prevent recording a manual owner review after
	// the workflow is resolved. No change to the incident verdict occurs.
	post(`{"action":"investigate"}`, http.StatusOK)
	post(`{"action":"resolve"}`, http.StatusOK)
	post(`{"action":"record_owner_review","summary":"Should not record on a resolved incident"}`, http.StatusConflict)
}
