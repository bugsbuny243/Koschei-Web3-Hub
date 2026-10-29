package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// Optional acceptance on a dedicated PostgreSQL test DB with migration 123.
// No insert, update, delete, DDL or incident creation occurs in this test.
func TestSOCRuntimeReadinessPostgresReadOnly(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("no isolated PostgreSQL configured: SOC read-path acceptance is NOT verified")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	h := &Handler{DB: db, DBRead: db}
	rec := httptest.NewRecorder()
	h.OwnerSOCReadiness(rec, httptest.NewRequest(http.MethodGet, "/api/owner/soc/readiness", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("PostgreSQL SOC read path did not verify: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK        bool                      `json:"ok"`
		Readiness ownerSOCReadinessSnapshot `json:"readiness"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || !body.Readiness.CasesTableReadable || !body.Readiness.ActionsTableReadable {
		t.Fatalf("read-only table verification did not complete: %#v", body)
	}
	if body.Readiness.SampledCases < 0 || body.Readiness.SampledCases > 50 {
		t.Fatalf("readiness sample limit violated: %d", body.Readiness.SampledCases)
	}
	if got := body.Readiness.State; got != socReadinessState(body.Readiness.SampledCases, body.Readiness.AnyAuditedActions) {
		t.Fatalf("unjustified readiness state: %q", got)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("readiness must not be cached")
	}
}
