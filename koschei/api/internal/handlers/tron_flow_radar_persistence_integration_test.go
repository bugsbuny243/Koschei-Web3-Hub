package handlers

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Requires the migrated PostgreSQL 17 release-gate database.
func TestPersistedTronFlowFindingPostgres17(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	key := "tron-flow:integration-" + time.Now().UTC().Format("20060102150405.000000000")
	_, err = db.ExecContext(ctx, `INSERT INTO security_alert_events
		(source, event_type, severity, target, title, message, dedupe_key, payload)
		VALUES ('tron_flow_radar', 'holder', 'high', 'TIntegrationAddress', 'Holder observation',
			'Observed funds', $1, $2::jsonb)`, key,
		`{"kind":"holder","observed_at":"2026-11-04T10:00:00Z","evidence":{"source":"postgres17","hops":2}}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := db.ExecContext(cleanup, "DELETE FROM security_alert_events WHERE dedupe_key = $1", key); err != nil {
			t.Errorf("cleanup alert: %v", err)
		}
	})
	h := &Handler{DBRead: db}
	finding, err := h.persistedTronFlowFinding(ctx, key)
	if err != nil {
		t.Fatalf("read persisted finding: %v", err)
	}
	if finding.DedupeKey != key || finding.Kind != "holder" || finding.Address != "TIntegrationAddress" ||
		finding.Severity != "high" || finding.ObservedAt.IsZero() || finding.Evidence["source"] != "postgres17" {
		t.Fatalf("unexpected persisted finding: %+v", finding)
	}
	if _, err := h.persistedTronFlowFinding(ctx, key+"-missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing finding error = %v, want sql.ErrNoRows", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE security_alert_events SET payload = $2::jsonb WHERE dedupe_key = $1",
		key, `{"kind":"holder","evidence":{"source":"postgres17"}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.persistedTronFlowFinding(ctx, key); err == nil {
		t.Fatal("malformed persisted payload was accepted")
	}
}
