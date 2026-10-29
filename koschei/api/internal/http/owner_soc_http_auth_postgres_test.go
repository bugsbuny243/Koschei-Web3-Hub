package http

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

	"koschei/api/internal/handlers"
)

// Exercises the registered owner routes and the real PostgreSQL incident
// ledger together. This does not create analyst accounts or change auth rules.
func TestOwnerSOCIncidentHTTPAuthBoundaryPostgres17(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
	if dsn == "" || os.Getenv("KOSCHEI_SOC_TEST_DB_CONFIRMATION") != "isolated" {
		t.Skip("isolated SOC PostgreSQL acceptance is not configured")
	}
	const isolatedDSN = "postgres://postgres:postgres@127.0.0.1:5432/koschei_soc_acceptance?sslmode=disable"
	if dsn != isolatedDSN {
		t.Fatal("HTTP owner acceptance requires the exact disposable loopback database DSN")
	}
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-test-private-owner-credential")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	t.Setenv("OWNER_WALLET", "soc-test-wallet")
	t.Setenv("KOSCHEI_OWNER_WALLET", "")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	title := fmt.Sprintf("soc-http-owner-acceptance-%d", time.Now().UnixNano())
	var id string
	err = db.QueryRow(`
		INSERT INTO security_incident_cases (title,severity,status,summary)
		VALUES ($1,'high','open','isolated owner HTTP test case')
		RETURNING id::text
	`, title).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})

	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{DB: db, DBRead: db}, t.TempDir())
	server := securityHeaders(mux)
	incidentPath := "/api/owner/incidents/" + id
	send := func(method, path, body, secret, wallet string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if secret != "" {
			r.Header.Set("x-koschei-secret", secret)
		}
		if wallet != "" {
			r.Header.Set("x-owner-wallet", wallet)
		}
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	verifyNoActionWrites := func() {
		t.Helper()
		var count int
		if err := db.QueryRow("SELECT count(*) FROM security_incident_actions WHERE incident_id=$1::uuid", id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("denied request modified incident audit: %d actions", count)
		}
		var status string
		if err := db.QueryRow("SELECT status FROM security_incident_cases WHERE id=$1::uuid", id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "open" {
			t.Fatalf("denied request changed case state to %q", status)
		}
	}

	// Anonymous callers, incorrect secrets, and wallet mismatches have no
	// read or write access through the actual registered HTTP routes.
	denied := []struct {
		name, method, path, body, secret, wallet string
	}{
		{"anonymous-case-read", http.MethodGet, incidentPath, "", "", ""},
		{"anonymous-queue-read", http.MethodGet, "/api/owner/incidents?triage=1", "", "", ""},
		{"anonymous-page", http.MethodGet, "/owner/soc-triage", "", "", ""},
		{"anonymous-create", http.MethodPost, "/api/owner/incidents", `{"title":"Unauthorized new incident"}`, "", ""},
		{"anonymous-note", http.MethodPost, incidentPath, `{"action":"record_owner_review","summary":"Unauthorized review attempt"}`, "", ""},
		{"anonymous-assign", http.MethodPost, incidentPath, `{"action":"assign_analyst","analyst_id":"fake-actor","tier":"L3"}`, "", ""},
		{"anonymous-escalation", http.MethodPost, incidentPath, `{"action":"request_escalation","tier":"L3","summary":"Unauthorized escalation attempt"}`, "", ""},
		{"incorrect-secret", http.MethodPost, incidentPath, `{"action":"record_owner_review","summary":"Wrong credential attempt"}`, "wrong-owner-secret", "soc-test-wallet"},
		{"secret-wallet-mismatch", http.MethodPost, incidentPath, `{"action":"record_owner_review","summary":"Wrong wallet attempt"}`, "soc-test-private-owner-credential", "different-wallet"},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(tc.method, tc.path, tc.body, tc.secret, tc.wallet)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("denied request returned %d, want 404: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), title) || strings.Contains(rec.Body.String(), "Owner SOC triage") {
				t.Fatal("private case data or page leaked through denied request")
			}
			verifyNoActionWrites()
		})
	}
	// Owner auth must also fail closed when its configured credential is absent.
	t.Setenv("OWNER_SECRET", "")
	noConfig := send(http.MethodPost, incidentPath,
		`{"action":"record_owner_review","summary":"Should not run without owner configuration"}`,
		"soc-test-private-owner-credential", "soc-test-wallet")
	if noConfig.Code != http.StatusNotFound {
		t.Fatalf("missing owner secret returned %d, want 404", noConfig.Code)
	}
	verifyNoActionWrites()
	t.Setenv("OWNER_SECRET", "soc-test-private-owner-credential")

	const goodSecret = "soc-test-private-owner-credential"
	const goodWallet = "soc-test-wallet"
	allowed := []struct {
		name, body string
	}{
		{"owner-review", `{"action":"record_owner_review","summary":"Owner examined original incident evidence references"}`},
		{"owner-assignment", `{"action":"assign_analyst","analyst_id":"soc-case-reviewer","tier":"L1"}`},
		{"owner-escalation", `{"action":"request_escalation","tier":"L2","summary":"Owner requests higher tier manual review"}`},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(http.MethodPost, incidentPath, tc.body, goodSecret, goodWallet)
			if rec.Code != http.StatusOK {
				t.Fatalf("authorized owner action returned %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
	var count, reviews, assignments, escalations int
	err = db.QueryRow(`
		SELECT COUNT(*)::int,
			COUNT(*) FILTER (WHERE payload ? 'soc_owner_review_v1')::int,
			COUNT(*) FILTER (WHERE payload ? 'soc_assignment_v1')::int,
			COUNT(*) FILTER (WHERE payload ? 'soc_escalation_v1')::int
		FROM security_incident_actions WHERE incident_id=$1::uuid
	`, id).Scan(&count, &reviews, &assignments, &escalations)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || reviews != 1 || assignments != 1 || escalations != 1 {
		t.Fatalf("owner HTTP audit inaccurate: total=%d review=%d assignment=%d escalation=%d",
			count, reviews, assignments, escalations)
	}

	rec := send(http.MethodGet, incidentPath, "", goodSecret, goodWallet)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner case read returned %d: %s", rec.Code, rec.Body.String())
	}
	var caseResult struct {
		OK      bool              `json:"ok"`
		Actions []json.RawMessage `json:"actions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &caseResult); err != nil {
		t.Fatal(err)
	}
	if !caseResult.OK || len(caseResult.Actions) != 3 {
		t.Fatalf("owner action history not persisted: %s", rec.Body.String())
	}
	queue := send(http.MethodGet, "/api/owner/incidents?triage=1&limit=100", "", goodSecret, goodWallet)
	if queue.Code != http.StatusOK {
		t.Fatalf("owner triage read returned %d: %s", queue.Code, queue.Body.String())
	}
	if !strings.Contains(queue.Body.String(), id) ||
		!strings.Contains(queue.Body.String(), "owner_manual_review_recorded") ||
		!strings.Contains(queue.Body.String(), "requested") ||
		!strings.Contains(queue.Body.String(), "not_configured_no_deadline_or_authenticated_assignment_claim") {
		t.Fatal("authenticated triage omitted persisted owner workflow metadata or truthful SLA boundary")
	}
	page := send(http.MethodGet, "/owner/soc-triage", "", goodSecret, goodWallet)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Owner SOC triage") {
		t.Fatalf("authenticated owner page unavailable: %d", page.Code)
	}
	if page.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner SOC page is cacheable")
	}
}
