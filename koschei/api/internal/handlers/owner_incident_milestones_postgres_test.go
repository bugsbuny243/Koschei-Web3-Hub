package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Requires an isolated PostgreSQL fixture database with migration 123.
// Never use a production DSN for KOSCHEI_TEST_DATABASE_URL.
func TestOwnerIncidentWorkflowMilestonesPostgres17(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is absent; DB acceptance not executed")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	title := fmt.Sprintf("soc-milestone-fixture-%d", time.Now().UnixNano())
	var id string
	err = db.QueryRow(`
		INSERT INTO security_incident_cases(title,severity,status,summary)
		VALUES ($1,'high','open','isolated milestone acceptance')
		RETURNING id::text
	`, title).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM security_incident_cases WHERE id=$1::uuid AND title=$2", id, title)
	})
	var createdAt time.Time
	if err := db.QueryRow("SELECT created_at FROM security_incident_cases WHERE id=$1::uuid", id).Scan(&createdAt); err != nil {
		t.Fatal(err)
	}
	insertAction := func(action string) {
		t.Helper()
		if _, err := db.Exec(`
			INSERT INTO security_incident_actions(incident_id,action_type,actor,summary,payload)
			VALUES($1::uuid,$2,'owner','milestone fixture','{}'::jsonb)
		`, id, action); err != nil {
			t.Fatal(err)
		}
	}
	insertAction("investigate")
	insertAction("contain")
	insertAction("resolve")
	ctx := context.Background()
	events, err := earliestOwnerIncidentMilestones(ctx, db, []ownerIncidentRecord{{ID: id, CreatedAt: createdAt}})
	if err != nil {
		t.Fatal(err)
	}
	actual := events[id]
	if actual == nil || actual.FirstInvestigationAt == nil || actual.FirstContainmentRecordAt == nil || actual.FirstResolutionRecordAt == nil {
		t.Fatalf("persisted workflow event timeline not projected: %#v", actual)
	}
	if actual.SecondsToFirstInvestigation == nil || *actual.SecondsToFirstInvestigation < 0 {
		t.Fatalf("invalid first recorded investigation duration: %#v", actual)
	}
	if actual.FirstReopenAt != nil {
		t.Fatal("reopen marker was fabricated")
	}
	var status, severity string
	if err := db.QueryRow("SELECT status,severity FROM security_incident_cases WHERE id=$1::uuid", id).Scan(&status, &severity); err != nil {
		t.Fatal(err)
	}
	if status != "open" || severity != "high" {
		t.Fatalf("read projection mutated operator case: status=%s severity=%s", status, severity)
	}
}
