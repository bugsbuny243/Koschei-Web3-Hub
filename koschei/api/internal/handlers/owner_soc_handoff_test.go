package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/socops"
)

func TestSOCHandoffRejectsIncompleteOrAlteredHistory(t *testing.T) {
	stamp := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	before := socops.State{Tier: "L1"}
	after := socops.State{Tier: "L1", Assignee: "Analyst", Version: 1}
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	event := socHandoffEvent{Version: 1, Action: "assign", Summary: "Test fixture", Before: b, After: a, CreatedAt: stamp}
	if err := validateSOCHandoffHistory(before, nil); err != nil {
		t.Fatal(err)
	}
	if err := validateSOCHandoffHistory(after, []socHandoffEvent{event}); err != nil {
		t.Fatal(err)
	}
	if validateSOCHandoffHistory(after, nil) == nil {
		t.Fatal("missing history accepted")
	}
	for _, mutate := range []func(*socHandoffEvent){
		func(e *socHandoffEvent) { e.Version = 2 },
		func(e *socHandoffEvent) { e.Action = "acknowledge" },
		func(e *socHandoffEvent) { e.Before = json.RawMessage(`{"tier":"L2"}`) },
		func(e *socHandoffEvent) { e.After = json.RawMessage(`null`) },
	} {
		bad := event
		mutate(&bad)
		if validateSOCHandoffHistory(after, []socHandoffEvent{bad}) == nil {
			t.Fatalf("invalid event accepted: %+v", bad)
		}
	}
	changed := after
	changed.Assignee = "Other"
	if validateSOCHandoffHistory(changed, []socHandoffEvent{event}) == nil {
		t.Fatal("mismatched current state accepted")
	}
}

// Called by the required PostgreSQL 17 operations acceptance test. The final
// limit fixtures only modify its isolated schema and never application data.
func testSOCHandoffExport(t *testing.T, h *Handler, id string, testLimits bool) {
	t.Helper()
	call := func(version string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.OwnerIncidentItem(rec, httptest.NewRequest("GET", "/api/owner/incidents/"+id+"?handoff="+version, nil))
		return rec
	}
	// A broken read replica must not prevent a primary-backed v2 export.
	badRead, err := sql.Open("postgres", "postgres://invalid:invalid@127.0.0.1:1/invalid?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer badRead.Close()
	originalRead := h.DBRead
	h.DBRead = badRead
	rec := call("2")
	h.DBRead = originalRead
	if rec.Code != 200 {
		t.Fatalf("SOC handoff: %d %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Schema  string          `json:"schema_version"`
		Content json.RawMessage `json:"content"`
		Digest  string          `json:"content_sha256"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(envelope.Content)
	if envelope.Schema != ownerIncidentHandoffSchemaV2 || hex.EncodeToString(sum[:]) != envelope.Digest {
		t.Fatal("v2 schema or digest mismatch")
	}
	var content ownerIncidentHandoffContent
	if err := json.Unmarshal(envelope.Content, &content); err != nil {
		t.Fatal(err)
	}
	op := content.Operations
	if op == nil || op.State.Version != 3 || op.State.Tier != "L2" || op.State.Assignee == "" || len(op.Events) != 3 || op.State.AcknowledgedAt == nil || op.InitialSeverity != "critical" || op.ReadSource != "primary_db" || op.SLA.RespondedAt == nil {
		t.Fatalf("missing SOC handoff content: %+v", op)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("private export headers missing")
	}
	legacy := call("1")
	if legacy.Code != 200 || strings.Contains(legacy.Body.String(), `"operations"`) {
		t.Fatal("legacy export changed")
	}
	if !testLimits {
		return
	}
	// Same snapshot must reject metadata not backed by its event chain.
	if _, err := h.DB.Exec(`UPDATE security_incident_soc SET version=4 WHERE incident_id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if rec := call("2"); rec.Code != 503 || strings.Contains(rec.Body.String(), "content_sha256") {
		t.Fatal("inconsistent history received digest")
	}
	if _, err := h.DB.Exec(`UPDATE security_incident_soc SET version=3 WHERE incident_id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	// Large JSON objects are bounded before history validation or hashing.
	if _, err := h.DB.Exec(`INSERT INTO security_incident_soc_events (incident_id,version,action,actor,summary,before_state,after_state)
 VALUES ($1::uuid,4,'assign','test','Oversize fixture',jsonb_build_object('large',repeat('x',131073)),'{}')`, id); err != nil {
		t.Fatal(err)
	}
	if rec := call("2"); rec.Code != 409 || strings.Contains(rec.Body.String(), "content_sha256") {
		t.Fatal("oversize event received digest")
	}
	// Use a separate incident so the count limit is exercised independently.
	var manyID string
	if err := h.DB.QueryRow(`INSERT INTO security_incident_cases (title) VALUES ('Explicit export count-limit fixture') RETURNING id::text`).Scan(&manyID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.DB.Exec(`INSERT INTO security_incident_soc_events (incident_id,version,action,actor,summary,before_state,after_state)
 SELECT $1::uuid,n,'assign','test','Count fixture','{}','{}' FROM generate_series(1,501) n`, manyID); err != nil {
		t.Fatal(err)
	}
	id = manyID
	if rec := call("2"); rec.Code != 409 || strings.Contains(rec.Body.String(), "content_sha256") {
		t.Fatal("truncated history received digest")
	}
}
