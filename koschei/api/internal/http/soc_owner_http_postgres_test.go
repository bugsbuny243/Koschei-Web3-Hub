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

	"koschei/api/internal/handlers"

	_ "github.com/lib/pq"
)

// Only the disposable runner-local SOC database may receive fixture writes.
func socHTTPAcceptanceDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
	if dsn == "" || os.Getenv("KOSCHEI_SOC_TEST_DB_CONFIRMATION") != "isolated" {
		t.Skip("isolated SOC HTTP database and explicit confirmation are required")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host := strings.ToLower(u.Hostname())
	if (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		(host != "127.0.0.1" && host != "localhost" && host != "::1") ||
		u.Port() != "5432" || u.Path != "/koschei_soc_acceptance" ||
		u.User == nil || u.User.Username() == "" || u.RawQuery != "sslmode=disable" || u.Fragment != "" {
		t.Fatal("refused SOC HTTP acceptance DSN outside disposable loopback test database")
	}
	return dsn
}

// Checks the registered routes and real ownerOnly middleware, not direct Handler calls.
func TestSOCIncidentOwnerAuthorizationPostgres17(t *testing.T) {
	dsn := socHTTPAcceptanceDSN(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "soc-http-owner-test-secret")
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
	title := fmt.Sprintf("soc-http-auth-acceptance-%d", time.Now().UnixNano())
	var id string
	if err := db.QueryRow(`
		INSERT INTO security_incident_cases(title,severity,status,summary)
		VALUES($1,'high','open','isolated HTTP owner authorization fixture') RETURNING id::text
	`, title).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})
	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{DB: db, DBRead: db}, t.TempDir())
	app := securityHeaders(mux)
	request := func(method, path, payload, secret string, extras map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(payload))
		if secret != "" {
			r.Header.Set("x-koschei-secret", secret)
		}
		if payload != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		for k, value := range extras {
			r.Header.Set(k, value)
		}
		app.ServeHTTP(rec, r)
		return rec
	}

	postURL := "/api/owner/incidents/" + id
	for _, operation := range []struct{ method, path, payload string }{
		{http.MethodGet, "/api/owner/incidents?triage=1&limit=5", ""},
		{http.MethodGet, postURL, ""},
		{http.MethodGet, postURL + "?handoff=2", ""},
		{http.MethodPost, postURL, `{"action":"assign_analyst","analyst_id":"soc-attacker","tier":"L3"}`},
		{http.MethodPost, postURL, `{"action":"record_owner_review","summary":"Forged owner review content"}`},
		{http.MethodPost, postURL, `{"action":"request_escalation","tier":"L3","summary":"Forged escalation request"}`},
		{http.MethodGet, "/owner/soc-triage", ""},
	} {
		for _, attempt := range []struct {
			label, secret string
			headers       map[string]string
		}{
			{"anonymous", "", nil},
			{"incorrect_secret", "incorrect-owner-secret", nil},
			{"spoofed_role", "", map[string]string{"X-Owner-Wallet": "spoofed-wallet", "X-Role": "owner", "X-Analyst-Tier": "L3"}},
			{"fake_bearer", "", map[string]string{"Authorization": "Bearer not-a-valid-owner-token"}},
		} {
			rec := request(operation.method, operation.path, operation.payload, attempt.secret, attempt.headers)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s %s unexpectedly allowed: HTTP %d %s", attempt.label, operation.method, operation.path, rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), title) || strings.Contains(rec.Body.String(), "Owner SOC triage") {
				t.Fatalf("%s leaked private owner SOC content", attempt.label)
			}
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM security_incident_actions WHERE incident_id=$1::uuid", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unauthorized requests wrote %d case audit events", count)
	}
	const ownerSecret = "soc-http-owner-test-secret"
	bundle := request(http.MethodGet, postURL+"?handoff=2", "", ownerSecret, nil)
	if bundle.Code != http.StatusOK || !strings.Contains(bundle.Body.String(), `"schema_version":"koschei.soc-incident-handoff.v2"`) || !strings.Contains(bundle.Body.String(), `"events":[]`) {
		t.Fatalf("authenticated v2 handoff failed: %d %s", bundle.Code, bundle.Body.String())
	}
	for _, payload := range []string{
		`{"action":"assign_analyst","analyst_id":"soc-verified-owner","tier":"L1"}`,
		`{"action":"request_escalation","tier":"L2","summary":"Owner requests senior review"}`,
		`{"action":"record_owner_review","summary":"Owner inspected incident provenance"}`,
	} {
		rec := request(http.MethodPost, postURL, payload, ownerSecret, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("authenticated owner action: HTTP %d %s", rec.Code, rec.Body.String())
		}
	}
	if err := db.QueryRow("SELECT count(*) FROM security_incident_actions WHERE incident_id=$1::uuid", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected three persisted owner audit actions, got %d", count)
	}
	rec := request(http.MethodGet, "/api/owner/incidents?triage=1&limit=250", "", ownerSecret, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authenticated read-only SOC queue failed: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		OK    bool `json:"ok"`
		Items []struct {
			Incident struct {
				ID string `json:"id"`
			} `json:"incident"`
			AnalystAssignment *struct {
				Status string `json:"status"`
			} `json:"analyst_assignment"`
			Escalation *struct {
				State string `json:"state"`
			} `json:"escalation"`
			OwnerReview *struct {
				EvidenceState string `json:"evidence_state"`
			} `json:"owner_review"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("owner case triage returned unconfirmed response")
	}
	for _, item := range result.Items {
		if item.Incident.ID != id {
			continue
		}
		if item.AnalystAssignment == nil || item.AnalystAssignment.Status != "assigned" ||
			item.Escalation == nil || item.Escalation.State != "requested" ||
			item.OwnerReview == nil || item.OwnerReview.EvidenceState != "owner_manual_review_recorded" {
			t.Fatalf("protected HTTP view missing persisted SOC workflow: %#v", item)
		}
		return
	}
	t.Fatal("test case not present in owner-only bounded SOC queue")
}
