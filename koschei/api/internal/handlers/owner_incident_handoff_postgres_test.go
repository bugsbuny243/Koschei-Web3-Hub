package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

func TestOwnerIncidentHandoffPostgresAcceptance(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL not configured; SOC handoff database acceptance remains unverified")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	title := fmt.Sprintf("soc-handoff-acceptance-%d", time.Now().UnixNano())
	var id string
	if err := db.QueryRow(`
		INSERT INTO security_incident_cases (title, status, severity, evidence_refs)
		VALUES ($1, 'open', 'high', '["evidence-ref-fixture"]'::jsonb)
		RETURNING id::text
	`, title).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Only this unique test fixture may be cleaned up.
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})
	_, err = db.Exec(`
		INSERT INTO security_incident_actions (incident_id, action_type, actor, summary, payload)
		VALUES ($1::uuid, 'note', 'owner', 'unit test only', '{"test_fixture":true}'::jsonb)
	`, id)
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{DB: db, DBRead: db}
	call := func() *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.OwnerIncidentItem(rec, httptest.NewRequest(http.MethodGet, "/api/owner/incidents/"+id+"?handoff=1", nil))
		return rec
	}
	rec := call()
	if rec.Code != http.StatusOK {
		t.Fatalf("handoff GET returned %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("handoff bundle must be private and non-cacheable")
	}
	var bundle struct {
		OK                 bool            `json:"ok"`
		Content            json.RawMessage `json:"content"`
		ContentSHA256      string          `json:"content_sha256"`
		EvidenceValidation string          `json:"evidence_validation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if !bundle.OK || bundle.EvidenceValidation != "not_performed_references_only" {
		t.Fatal("bundle must not claim independent evidence validation")
	}
	sum := sha256.Sum256(bundle.Content)
	if hex.EncodeToString(sum[:]) != bundle.ContentSHA256 {
		t.Fatal("handoff digest does not match serialized response content")
	}
	var snapshot ownerIncidentHandoffContent
	if err := json.Unmarshal(bundle.Content, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Incident.ID != id || len(snapshot.Actions) != 1 ||
		len(snapshot.Incident.EvidenceRefs) != 1 {
		t.Fatalf("unexpected incident snapshot %#v", snapshot)
	}

	// Overlimit history must fail closed. Never publish a silently truncated hash.
	if _, err := db.Exec(`
		INSERT INTO security_incident_actions (incident_id, action_type, actor, summary, payload)
		SELECT $1::uuid, 'note', 'owner', 'generated test fixture', '{}'::jsonb
		FROM generate_series(1, 500)
	`, id); err != nil {
		t.Fatal(err)
	}
	rec = call()
	if rec.Code != http.StatusConflict ||
		!strings.Contains(rec.Body.String(), "more than 500") {
		t.Fatalf("overlimit handoff should fail with 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "content_sha256") {
		t.Fatal("truncated audit history must not be issued with a hash")
	}
}
