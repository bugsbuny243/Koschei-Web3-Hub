package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	if got.ChainEvidenceVerified || got.PayloadHashScope != "sha256_of_postgresql_jsonb_text_read_snapshot_not_source_signature" {
		t.Fatalf("alert hash was promoted to chain proof: %#v", got)
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
	initialHash := sha256.Sum256([]byte("{}"))
	if before[0].StoredPayloadSHA256 != hex.EncodeToString(initialHash[:]) || before[0].ChainEvidenceVerified || before[0].CreatedAt.IsZero() {
		t.Fatalf("DB provenance proof cannot be reconstructed: %#v", before[0])
	}
	if _, err := db.ExecContext(ctx, `UPDATE security_alert_events SET payload=$2::jsonb WHERE id=$1::uuid`,
		highID, `{"internal_trace":"fixture-private-marker"}`); err != nil {
		t.Fatal(err)
	}
	var storedPayload string
	if err := db.QueryRowContext(ctx, "SELECT payload::text FROM security_alert_events WHERE id=$1::uuid", highID).Scan(&storedPayload); err != nil {
		t.Fatal(err)
	}
	storedHash := sha256.Sum256([]byte(storedPayload))
	changed := fetch()
	if len(changed) != 1 || changed[0].StoredPayloadSHA256 != hex.EncodeToString(storedHash[:]) || changed[0].ChainEvidenceVerified {
		t.Fatalf("persisted JSONB digest not tracked correctly: %#v", changed)
	}
	out, err := json.Marshal(changed[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "fixture-private-marker") || strings.Contains(string(out), "internal_trace") {
		t.Fatalf("raw alert payload escaped the provenance projection: %s", out)
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
