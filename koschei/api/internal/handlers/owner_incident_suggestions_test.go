package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestOwnerIncidentReviewCandidateIsNotAnIncident(t *testing.T) {
	alert := ownerIncidentAlertSource{
		ID: "alert-1", Source: "global_radar_runtime",
		EventType: "global_radar.coverage.attention", Severity: "high",
		Target: "ethereum-mainnet", Title: strings.Repeat("Alert ", 50),
		OccurrenceCount: 2, LastSeenAt: time.Now().UTC(),
	}
	got := ownerIncidentReviewCandidateFromAlert(alert)
	if got.ReviewState != "owner_review_required" || got.EvidenceState != "persisted_alert_not_independent_evidence_verification" {
		t.Fatalf("review evidence state was promoted: %#v", got)
	}
	if got.SuggestedIncident.Network != "" {
		t.Fatalf("network was guessed from target: %#v", got.SuggestedIncident)
	}
	if got.SuggestedIncident.Severity != "high" || len(got.SuggestedIncident.AlertRefs) != 1 || got.SuggestedIncident.AlertRefs[0] != alert.ID {
		t.Fatalf("lost existing alert evidence binding: %#v", got)
	}
	if len([]rune(got.SuggestedIncident.Title)) > 180 {
		t.Fatalf("suggestion title exceeds existing incident limit: %#v", got.SuggestedIncident.Title)
	}
	if len(got.SuggestedIncident.EvidenceRefs) != 0 || len(got.SuggestedIncident.DossierRefs) != 0 {
		t.Fatalf("suggestion invented unrelated evidence: %#v", got.SuggestedIncident)
	}
}

func TestOwnerIncidentSuggestionsUnavailableCannotPretendToSucceed(t *testing.T) {
	h := &Handler{}
	res := httptest.NewRecorder()
	h.OwnerIncidents(res, httptest.NewRequest(http.MethodGet, "/api/owner/incidents?suggestions=1", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing DB returned status %d: %s", res.Code, res.Body.String())
	}
	defaultRes := httptest.NewRecorder()
	h.OwnerIncidents(defaultRes, httptest.NewRequest(http.MethodGet, "/api/owner/incidents", nil))
	if defaultRes.Code != http.StatusServiceUnavailable {
		t.Fatalf("default incident behavior changed: %d %s", defaultRes.Code, defaultRes.Body.String())
	}
}

func TestOwnerIncidentSuggestionsReviewAndExplicitCreationPostgres17(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	idSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	target := "review-candidate-" + idSuffix
	var highID, mediumID string
	insert := func(severity string) string {
		t.Helper()
		var id string
		err := db.QueryRowContext(ctx, `
			INSERT INTO security_alert_events
				(source, event_type, severity, target, title, message, dedupe_key, payload)
			VALUES ('arvis', 'arvis.verdict.created', $1, $2, $3,
			        'test alert', $4, '{}'::jsonb)
			RETURNING id::text
		`, severity, target, severity+" candidate "+idSuffix,
			"incident-review-test:"+severity+":"+idSuffix).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	highID = insert("high")
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_alert_events WHERE id=$1::uuid", highID)
	})
	mediumID = insert("medium")
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_alert_events WHERE id=$1::uuid", mediumID)
	})
	h := &Handler{DB: db, DBRead: db}
	fetch := func() []ownerIncidentReviewCandidate {
		t.Helper()
		res := httptest.NewRecorder()
		h.OwnerIncidents(res, httptest.NewRequest(http.MethodGet,
			"/api/owner/incidents?suggestions=1&target="+url.QueryEscape(target)+"&limit=100", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("read status=%d: %s", res.Code, res.Body.String())
		}
		if res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("operator review suggestions must not be cached")
		}
		var answer struct {
			SchemaVersion string                         `json:"schema_version"`
			Suggestions   []ownerIncidentReviewCandidate `json:"suggestions"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &answer); err != nil {
			t.Fatal(err)
		}
		if answer.SchemaVersion != ownerIncidentSuggestionSchema {
			t.Fatalf("unexpected schema %q", answer.SchemaVersion)
		}
		return answer.Suggestions
	}
	before := fetch()
	if len(before) != 1 || before[0].AlertID != highID {
		t.Fatalf("high/medium separation or source resolution failed: %#v", before)
	}
	var caseCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM security_incident_cases WHERE target=$1", target).Scan(&caseCount); err != nil || caseCount != 0 {
		t.Fatalf("read-only projection mutated incident state: count=%d err=%v", caseCount, err)
	}
	payload, err := json.Marshal(before[0].SuggestedIncident)
	if err != nil {
		t.Fatal(err)
	}
	createRes := httptest.NewRecorder()
	h.OwnerIncidents(createRes, httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(string(payload))))
	if createRes.Code != http.StatusCreated {
		t.Fatalf("manual incident creation failed: %d %s", createRes.Code, createRes.Body.String())
	}
	var created struct {
		Incident ownerIncidentRecord `json:"incident"`
	}
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Incident.ID == "" {
		t.Fatal("manual creation returned no incident")
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_incident_cases WHERE id=$1::uuid", created.Incident.ID)
	})
	after := fetch()
	if len(after) != 0 {
		t.Fatalf("linked active incident still offered as new candidate: %#v", after)
	}
}
