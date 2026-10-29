package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// This test is opt-in and must run ONLY against an isolated PostgreSQL fixture
// database with migration 123 applied. It never targets an actual customer case.
func TestOwnerReviewAttentionPolicyPostgresAcceptance(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("KOSCHEI_TEST_SOC_DB_ISOLATED") != "1" {
		t.Skip("isolated PostgreSQL acceptance not configured; NOT VERIFIED")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	title := fmt.Sprintf("soc-review-policy-fixture-%d", now.UnixNano())
	createdAt := now.Add(-time.Hour)
	var id string
	if err := db.QueryRow(`
		INSERT INTO security_incident_cases (title, severity, status, summary, created_at, updated_at)
		VALUES ($1,'critical','open','isolated review attention test', $2, now())
		RETURNING id::text
	`, title, createdAt).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})
	h := &Handler{DB: db, DBRead: db}
	serve := func(policy string) (int, []byte) {
		t.Helper()
		t.Setenv(ownerIncidentReviewPolicyEnv, policy)
		rec := httptest.NewRecorder()
		h.OwnerIncidents(rec, httptest.NewRequest(http.MethodGet, "/api/owner/incidents?triage=1&status=open&limit=250", nil))
		return rec.Code, rec.Body.Bytes()
	}
	type response struct {
		ReviewPolicyState string `json:"review_policy_state"`
		SLAState          string `json:"sla_state"`
		Items             []struct {
			Incident struct {
				ID           string   `json:"id"`
				EvidenceRefs []string `json:"evidence_refs"`
			} `json:"incident"`
			ReviewAttention *ownerIncidentReviewAttention `json:"review_attention"`
		} `json:"items"`
	}
	find := func(data response) *ownerIncidentReviewAttention {
		for _, item := range data.Items {
			if item.Incident.ID == id {
				return item.ReviewAttention
			}
		}
		t.Fatal("fixture incident missing from bounded owner queue")
		return nil
	}

	code, payload := serve("")
	if code != http.StatusOK {
		t.Fatalf("unconfigured policy must preserve owner queue: %d %s", code, payload)
	}
	var noPolicy response
	if err := json.Unmarshal(payload, &noPolicy); err != nil {
		t.Fatal(err)
	}
	if noPolicy.ReviewPolicyState != "disabled_unconfigured" || find(noPolicy) != nil {
		t.Fatal("unconfigured policy created a review deadline")
	}

	code, payload = serve("{broken")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("invalid configured policy must fail closed, got %d %s", code, payload)
	}

	code, payload = serve(validReviewPolicyJSON(now.Add(-2 * time.Hour).Format(time.RFC3339)))
	if code != http.StatusOK {
		t.Fatalf("configured policy returned %d: %s", code, payload)
	}
	var active response
	if err := json.Unmarshal(payload, &active); err != nil {
		t.Fatal(err)
	}
	attention := find(active)
	if active.ReviewPolicyState != "configured_operational_review_only" ||
		attention == nil || attention.State != "review_window_elapsed_not_sla_breach" ||
		attention.OverSeconds == nil || *attention.OverSeconds <= 0 {
		t.Fatalf("persisted case was not measured honestly: %#v", attention)
	}
	if active.SLAState != "not_configured_no_deadline_or_authenticated_assignment_claim" {
		t.Fatalf("review hint fabricated SLA authority: %q", active.SLAState)
	}
	var actionCount int
	if err := db.QueryRow("SELECT count(*) FROM security_incident_actions WHERE incident_id=$1::uuid", id).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 0 {
		t.Fatalf("read-only review projection unexpectedly wrote %d audit actions", actionCount)
	}
}
