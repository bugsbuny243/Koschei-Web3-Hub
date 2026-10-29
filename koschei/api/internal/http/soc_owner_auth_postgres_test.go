package http

import (
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

	"koschei/api/internal/handlers"
)

// This test exercises the real ownerOnly + requiresDB + incident HTTP routes,
// rather than invoking incident handlers directly. It never contacts a live
// service and writes only to the disposable PostgreSQL 17 CI fixture database.
func TestOwnerIncidentHTTPAuthPostgres17(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
	if dsn == "" || os.Getenv("KOSCHEI_SOC_TEST_DB_CONFIRMATION") != "isolated" {
		t.Skip("SOC HTTP auth acceptance needs an explicitly isolated PostgreSQL database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("invalid SOC test DSN: %v", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") ||
		(host != "127.0.0.1" && host != "localhost" && host != "::1") ||
		parsed.Port() != "5432" || parsed.Path != "/koschei_soc_acceptance" ||
		parsed.User == nil || parsed.User.Username() == "" ||
		parsed.RawQuery != "sslmode=disable" || parsed.Fragment != "" {
		t.Fatal("SOC auth test refuses any non-disposable PostgreSQL target")
	}

	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-owner-http-acceptance-only-secret")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	t.Setenv("OWNER_WALLET", "")
	t.Setenv("KOSCHEI_OWNER_WALLET", "")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	title := fmt.Sprintf("soc-http-auth-%d", time.Now().UnixNano())
	var id string
	if err := db.QueryRow(`
		INSERT INTO security_incident_cases (title, severity, status, summary)
		VALUES ($1, 'high', 'open', 'disposable owner HTTP auth fixture')
		RETURNING id::text
	`, title).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// This fixture is unique to the test; its audit actions cascade.
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})

	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{DB: db, DBRead: db}, t.TempDir())
	server := securityHeaders(mux)
	base := "/api/owner/incidents/" + id
	do := func(method, path, body, credential string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if credential != "" {
			req.Header.Set("x-koschei-secret", credential)
		}
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "soc-owner-http-acceptance-only-secret") {
			t.Fatal("owner credential leaked into HTTP response")
		}
		return rec
	}

	const assign = `{"action":"assign_analyst","analyst_id":"soc-l1-fixture","tier":"L1"}`
	const review = `{"action":"record_owner_review","summary":"Owner checked this fixture incident without a chain verdict"}`
	const escalate = `{"action":"request_escalation","tier":"L2","summary":"Escalate this fixture for manual review"}`
	for _, path := range []string{base, "/api/owner/incidents?triage=1&limit=100", "/owner/soc-triage"} {
		for _, credential := range []string{"", "wrong-fixture-secret"} {
			rec := do(http.MethodGet, path, "", credential)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("unauthorized GET %s returned %d, want 404: %s", path, rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), title) {
				t.Fatal("unauthorized caller received incident title")
			}
		}
	}
	for _, body := range []string{assign, review, escalate} {
		for _, credential := range []string{"", "wrong-fixture-secret"} {
			rec := do(http.MethodPost, base, body, credential)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("unauthorized POST returned %d, want 404: %s", rec.Code, rec.Body.String())
			}
		}
	}
	// A forged Bearer value does not substitute for verified owner credentials.
	req := httptest.NewRequest(http.MethodPost, base, strings.NewReader(assign))
	req.Header.Set("Authorization", "Bearer untrusted-customer-jwt")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("forged Bearer caller was granted incident write: HTTP %d", rec.Code)
	}
	var beforeCount int
	var beforeStatus string
	if err := db.QueryRow(`
		SELECT count(*) FROM security_incident_actions WHERE incident_id=$1::uuid
	`, id).Scan(&beforeCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT status FROM security_incident_cases WHERE id=$1::uuid", id).Scan(&beforeStatus); err != nil {
		t.Fatal(err)
	}
	if beforeCount != 0 || beforeStatus != "open" {
		t.Fatalf("unauthorized requests changed state: actions=%d status=%q", beforeCount, beforeStatus)
	}

	const ownerCredential = "soc-owner-http-acceptance-only-secret"
	for _, body := range []string{assign, review, escalate} {
		rec := do(http.MethodPost, base, body, ownerCredential)
		if rec.Code != http.StatusOK {
			t.Fatalf("owner HTTP action failed %d: %s", rec.Code, rec.Body.String())
		}
	}
	rec = do(http.MethodGet, base, "", ownerCredential)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner incident audit read failed %d: %s", rec.Code, rec.Body.String())
	}
	var timeline struct {
		OK      bool `json:"ok"`
		Actions []struct {
			ActionType string         `json:"action_type"`
			Actor      string         `json:"actor"`
			Payload    map[string]any `json:"payload"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &timeline); err != nil {
		t.Fatal(err)
	}
	if !timeline.OK || len(timeline.Actions) != 3 {
		t.Fatalf("owner timeline is incomplete: %#v", timeline)
	}
	for _, action := range timeline.Actions {
		if action.ActionType != "note" || action.Actor != "owner" {
			t.Fatalf("unexpected owner audit actor/action: %#v", action)
		}
	}
	rec = do(http.MethodGet, "/api/owner/incidents?triage=1&limit=100", "", ownerCredential)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner triage read failed %d: %s", rec.Code, rec.Body.String())
	}
	var queue struct {
		OK    bool `json:"ok"`
		Items []struct {
			Incident struct {
				ID string `json:"id"`
			} `json:"incident"`
			Assignment *struct {
				Status string `json:"status"`
				Tier   string `json:"tier"`
			} `json:"analyst_assignment"`
			Escalation *struct {
				State      string `json:"state"`
				TargetTier string `json:"target_tier"`
			} `json:"escalation"`
			OwnerReview *struct {
				EvidenceState string `json:"evidence_state"`
				RecordCount   int64  `json:"record_count"`
			} `json:"owner_review"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range queue.Items {
		if item.Incident.ID != id {
			continue
		}
		found = true
		if item.Assignment == nil || item.Assignment.Status != "assigned" || item.Assignment.Tier != "L1" ||
			item.Escalation == nil || item.Escalation.State != "requested" || item.Escalation.TargetTier != "L2" ||
			item.OwnerReview == nil || item.OwnerReview.EvidenceState != "owner_manual_review_recorded" ||
			item.OwnerReview.RecordCount != 1 {
			t.Fatalf("owner HTTP triage did not return persisted audit state: %#v", item)
		}
	}
	if !queue.OK || !found {
		t.Fatalf("owner HTTP triage did not contain the newly audited fixture: %#v", queue)
	}
	// The owner HTML endpoint itself is still a read-only surface.
	rec = do(http.MethodPost, "/owner/soc-triage", assign, ownerCredential)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("owner triage HTML was writable: HTTP %d", rec.Code)
	}
	var actionCount int
	if err := db.QueryRow("SELECT count(*) FROM security_incident_actions WHERE incident_id=$1::uuid", id).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 3 {
		t.Fatalf("unauthorized or read-only requests wrote unexpected actions: count=%d", actionCount)
	}
}
