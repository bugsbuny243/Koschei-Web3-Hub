package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "github.com/lib/pq"
)

// Uses an isolated test schema; never points at or cleans up application tables.
func TestSOCOperationsPostgres(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var unique string
	if err := admin.QueryRow(`SELECT replace(gen_random_uuid()::text,'-','')`).Scan(&unique); err != nil {
		t.Fatal(err)
	}
	schema := "soc_test_" + unique
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"123_security_center_incident_workflow.sql", "137_soc_operations.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		observed := probeSOCOperations(context.Background(), db)
		if name == "123_security_center_incident_workflow.sql" && observed.State != "unavailable_not_accepted" {
			t.Fatalf("missing migration claimed ready: %+v", observed)
		}
		if name == "137_soc_operations.sql" && (!observed.MetadataReadable || !observed.HistoryReadable || observed.State != "schema_read_observed_not_write_accepted") {
			t.Fatalf("operations schema unreadable: %+v", observed)
		}
	}
	var id string
	if err := db.QueryRow(`INSERT INTO security_incident_cases (title,severity,created_at) VALUES ('Explicit SOC test fixture','critical',now()-interval '2 hours') RETURNING id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO security_incident_actions (incident_id,action_type,payload,created_at) SELECT id,'created','{"severity":"critical"}',created_at FROM security_incident_cases WHERE id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	h := &Handler{DB: db, DBRead: db}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		if strings.HasPrefix(path, "/api/owner/soc/queue") {
			h.OwnerSOCQueue(w, r)
		} else {
			h.OwnerSOCCase(w, r)
		}
		return w
	}
	casePath := "/api/owner/soc/case?id=" + id
	readCase := func() map[string]any {
		t.Helper()
		w := request("GET", casePath, "")
		if w.Code != 200 {
			t.Fatalf("read: %d %s", w.Code, w.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	initial := readCase()["incident"].(map[string]any)
	if initial["sla"].(map[string]any)["state"] != "overdue" || initial["operations"].(map[string]any)["version"] != float64(0) {
		t.Fatal(initial)
	}
	// Two editors race with revision zero: exactly one update and audit event wins.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, label := range []string{"Analyst A", "Analyst B"} {
		wg.Add(1)
		go func(label string) {
			defer wg.Done()
			w := request("POST", casePath, fmt.Sprintf(`{"action":"assign","assignee":%q,"summary":"Test handoff","expected_version":0}`, label))
			codes <- w.Code
		}(label)
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		if code == 200 {
			success++
		} else if code == 409 {
			conflict++
		} else {
			t.Fatalf("race status %d", code)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	for _, command := range []string{
		`{"action":"escalate","tier":"L2","summary":"Specialist requested","expected_version":1}`,
		`{"action":"acknowledge","summary":"Response recorded","expected_version":2}`,
	} {
		w := request("POST", casePath, command)
		if w.Code != 200 {
			t.Fatalf("action %d: %s", w.Code, w.Body.String())
		}
	}
	after := readCase()
	item := after["incident"].(map[string]any)
	if item["sla"].(map[string]any)["state"] != "breached" || len(after["events"].([]any)) != 3 {
		t.Fatal(after)
	}
	// Initial severity is immutable history; changing current severity cannot reset SLA.
	if _, err := db.Exec(`UPDATE security_incident_cases SET severity='info' WHERE id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if readCase()["incident"].(map[string]any)["sla"].(map[string]any)["state"] != "breached" {
		t.Fatal("reprioritization hid breach")
	}
	// Existing incident response is preferred if it predates explicit acknowledgement.
	if _, err := db.Exec(`INSERT INTO security_incident_actions (incident_id,action_type,created_at) SELECT id,'investigate',created_at+interval '1 minute' FROM security_incident_cases WHERE id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if readCase()["incident"].(map[string]any)["sla"].(map[string]any)["state"] != "met" {
		t.Fatal("legacy response was ignored")
	}
	w := request("GET", "/api/owner/soc/queue?tier=L2&limit=1", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatal(w.Body.String())
	}
	w = request("GET", "/api/owner/soc/queue?unassigned=1", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), id) {
		t.Fatal("assignment filter failed")
	}
	for _, path := range []string{"/api/owner/soc/queue?tier=L4", "/api/owner/soc/queue?offset=-1", "/api/owner/soc/case?id=invalid"} {
		if w := request("GET", path, ""); w.Code != 400 {
			t.Fatalf("accepted %s", path)
		}
	}
	for _, body := range []string{`{"action":"assign","summary":"x"}`, `{"action":"assign","summary":"x","expected_version":3,"actor":"forged"}`, `{"action":"acknowledge","summary":"x","expected_version":3} {}`} {
		if w := request("POST", casePath, body); w.Code != 400 {
			t.Fatalf("accepted %s: %d", body, w.Code)
		}
	}
	for _, query := range []string{`UPDATE security_incident_soc_events SET summary='rewritten' WHERE incident_id=$1::uuid`, `DELETE FROM security_incident_soc_events WHERE incident_id=$1::uuid`} {
		if _, err := db.Exec(query, id); err == nil {
			t.Fatal("audit mutation was not blocked")
		}
	}
	// The SOC layer has not changed source incident status, evidence or action count.
	var status string
	var actions int
	if err := db.QueryRow(`SELECT status,(SELECT count(*) FROM security_incident_actions WHERE incident_id=c.id) FROM security_incident_cases c WHERE id=$1::uuid`, id).Scan(&status, &actions); err != nil {
		t.Fatal(err)
	}
	if status != "open" || actions != 2 {
		t.Fatalf("source workflow changed: %s %d", status, actions)
	}
	if _, err := db.Exec(`UPDATE security_incident_cases SET status='closed' WHERE id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", casePath, `{"action":"assign","assignee":"C","summary":"Closed case","expected_version":3}`); w.Code != 409 {
		t.Fatal(w.Body.String())
	}
}
