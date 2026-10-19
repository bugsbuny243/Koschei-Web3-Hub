package services

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestArvisStreamCanonicalClaimOutcomeRejectsUnverifiedCompletion(t *testing.T) {
	ctx := context.Background()
	if done, err := markArvisStreamNonFinalClaim(ctx, nil, "stream-id", "failed", ""); done || err == nil {
		t.Fatalf("missing writer unexpectedly accepted: done=%v err=%v", done, err)
	}
	// Even an authorized caller cannot use fallback status to create a
	// completed final; only the authenticated PostgreSQL final writer may.
	if done, err := markArvisStreamNonFinalClaim(ctx, nil, "stream-id", "completed", ""); done || err == nil {
		t.Fatalf("unverified completion unexpectedly accepted: done=%v err=%v", done, err)
	}
}

func TestArvisStreamCanonicalClaimOutcomePostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() // test fixtures must never become production events

	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality)
		VALUES('ci_fixture','test',$1,$2,'test_signed_final_claim_outcome',$3,'token',$4,'fixture_only')
		RETURNING id::text
	`, event.Network, event.ModuleID, event.Target, event.Signature).Scan(&event.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO arvis_stream_processing(stream_event_id,target,signature,status,attempts)
		VALUES($1::uuid,$2,$3,'processing',1)
	`, event.ID, event.Target, event.Signature); err != nil {
		t.Fatal(err)
	}
	status := func() string {
		t.Helper()
		var got string
		if err := tx.QueryRowContext(ctx, `
			SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid
		`, event.ID).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	if done, err := markArvisStreamNonFinalClaim(ctx, tx, event.ID, "completed", "no signature"); done || err == nil {
		t.Fatalf("fallback attempted unsigned completion: done=%v err=%v", done, err)
	}
	if got := status(); got != "processing" {
		t.Fatalf("unauthorized completion changed processing state to %q", got)
	}
	if done, err := markArvisStreamNonFinalClaim(ctx, tx, event.ID, "insufficient_evidence", "missing evidence"); !done || err != nil {
		t.Fatalf("genuine insufficiency refused: done=%v err=%v", done, err)
	}
	if got := status(); got != "insufficient_evidence" {
		t.Fatalf("insufficient evidence status=%q", got)
	}
	if done, err := markArvisStreamNonFinalClaim(ctx, tx, event.ID, "failed", "late async worker error"); done || err != nil {
		t.Fatalf("prior terminal outcome downgraded: done=%v err=%v", done, err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE arvis_stream_processing
		SET status='processing',processed_at=NULL,updated_at=now()
		WHERE stream_event_id=$1::uuid
	`, event.ID); err != nil {
		t.Fatal(err)
	}
	if done, err := markArvisStreamNonFinalClaim(ctx, tx, event.ID, "failed", "temporary RPC failure"); !done || err != nil {
		t.Fatalf("unbacked failure could not be recorded: done=%v err=%v", done, err)
	}
	if got := status(); got != "failed" {
		t.Fatalf("retryable failure status=%q", got)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE arvis_stream_processing SET status='processing',updated_at=now()
		WHERE stream_event_id=$1::uuid
	`, event.ID); err != nil {
		t.Fatal(err)
	}

	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, verdict, registry); err != nil {
		t.Fatalf("trusted signed final and atomic completion failed: %v", err)
	}
	if got := status(); got != "completed" {
		t.Fatalf("signed final not completed: %q", got)
	}
	for _, fallback := range []string{"failed", "insufficient_evidence"} {
		if done, err := markArvisStreamNonFinalClaim(ctx, tx, event.ID, fallback, "late worker result"); done || err != nil {
			t.Fatalf("fallback %q downgraded signed completion: done=%v err=%v", fallback, done, err)
		}
		if got := status(); got != "completed" {
			t.Fatalf("signed final changed to %q", got)
		}
	}

	// Even an abnormal stale 'processing' state with an already persisted
	// signed receipt cannot be overwritten by an unrelated late error.
	if _, err := tx.ExecContext(ctx, `
		UPDATE arvis_stream_processing
		SET status='processing',processed_at=NULL,updated_at=now()-interval '10 minutes'
		WHERE stream_event_id=$1::uuid
	`, event.ID); err != nil {
		t.Fatal(err)
	}
	for _, fallback := range []string{"failed", "insufficient_evidence"} {
		if done, err := markArvisStreamNonFinalClaim(ctx, tx, event.ID, fallback, "older callback"); done || err != nil {
			t.Fatalf("receipt-backed claim was overwritten: status=%s done=%v err=%v", fallback, done, err)
		}
	}
	if got := status(); got != "processing" {
		t.Fatalf("canonical recovery opportunity lost to %q", got)
	}
	if done, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, event.ID, registry); !done || err != nil {
		t.Fatalf("trusted recovery could not finish receipt-backed claim: done=%v err=%v", done, err)
	}
	if got := status(); got != "completed" {
		t.Fatalf("signed final recovery outcome=%q", got)
	}
	// Sanitization must remain bounded before inserting into the queue.
	if got := compactArvisWorkerError(strings.Repeat("a", 600)); len(got) != 500 {
		t.Fatalf("unbounded error detail length %d", len(got))
	}
}
