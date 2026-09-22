package services

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestSovereignEnrichmentClaimPrioritizesNewestJournalEventPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		olderSignature = "ci-sovereign-live-priority-older"
		newerSignature = "ci-sovereign-live-priority-newer"
		eventType      = "ci_sovereign_live_priority"
	)
	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM security_radar_stream_events
			WHERE event_type=$1
			  AND signature IN ($2,$3)
		`, eventType, olderSignature, newerSignature)
	}
	cleanup()
	defer cleanup()

	_, err = db.ExecContext(ctx, `
		INSERT INTO security_radar_stream_events
			(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality,decoded,raw_event,created_at,updated_at)
		VALUES
			('ci','journal','solana-mainnet',$1,$2,$3,'token_or_launch',$3,'raw_stream','{}'::jsonb,'{}'::jsonb,TIMESTAMPTZ '9999-01-01 00:00:00+00',now()),
			('ci','journal','solana-mainnet',$1,$2,$4,'token_or_launch',$4,'raw_stream','{}'::jsonb,'{}'::jsonb,TIMESTAMPTZ '9999-01-02 00:00:00+00',now())
	`, ModulePumpSybilRadar, eventType, olderSignature, newerSignature)
	if err != nil {
		t.Fatal(err)
	}

	worker := &securityRadarJournalStreamWorker{
		Store: NewSecurityRadarStore(db),
	}
	claimed, err := worker.claimEnrichmentBatch(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 {
		t.Fatalf("claimed=%d want=1", len(claimed))
	}
	if claimed[0].Signature != newerSignature {
		t.Fatalf("claimed signature=%q want newest %q", claimed[0].Signature, newerSignature)
	}

	var olderStatus string
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(decoded->>'sovereign_enrichment_status','')
		FROM security_radar_stream_events
		WHERE signature=$1 AND event_type=$2
	`, olderSignature, eventType).Scan(&olderStatus); err != nil {
		t.Fatal(err)
	}
	if olderStatus != "" {
		t.Fatalf("older event was claimed by live worker: status=%q", olderStatus)
	}
}


func TestSovereignEnrichmentClaimSkipsFailedWSSJournalEventPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		failedSignature  = "ci-sovereign-failed-wss-newer"
		successSignature = "ci-sovereign-success-wss-older"
		eventType        = "ci_sovereign_failed_wss_skip"
	)
	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM security_radar_stream_events
			WHERE event_type=$1
			  AND signature IN ($2,$3)
		`, eventType, failedSignature, successSignature)
	}
	cleanup()
	defer cleanup()

	_, err = db.ExecContext(ctx, `
		INSERT INTO security_radar_stream_events
			(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality,decoded,raw_event,created_at,updated_at)
		VALUES
			('ci','journal','solana-mainnet',$1,$2,$3,'token_or_launch',$3,'raw_stream',
			 '{"err":null}'::jsonb,'{}'::jsonb,TIMESTAMPTZ '9999-02-01 00:00:00+00',now()),
			('ci','journal','solana-mainnet',$1,$2,$4,'token_or_launch',$4,'raw_stream',
			 '{"err":{"InstructionError":[0,"Custom"]},"sovereign_enrichment_status":"skipped_failed_transaction","sovereign_enrichment_skip_reason":"wss_transaction_failed"}'::jsonb,
			 '{}'::jsonb,TIMESTAMPTZ '9999-02-02 00:00:00+00',now())
	`, ModulePumpSybilRadar, eventType, successSignature, failedSignature)
	if err != nil {
		t.Fatal(err)
	}

	worker := &securityRadarJournalStreamWorker{Store: NewSecurityRadarStore(db)}
	claimed, err := worker.claimEnrichmentBatch(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 {
		t.Fatalf("claimed=%d want=1", len(claimed))
	}
	if claimed[0].Signature != successSignature {
		t.Fatalf("claimed signature=%q want successful %q", claimed[0].Signature, successSignature)
	}

	var failedStatus string
	var failedAttempts int
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(decoded->>'sovereign_enrichment_status',''),
		       COALESCE((decoded->>'sovereign_enrichment_attempts')::integer,0)
		FROM security_radar_stream_events
		WHERE signature=$1 AND event_type=$2
	`, failedSignature, eventType).Scan(&failedStatus, &failedAttempts); err != nil {
		t.Fatal(err)
	}
	if failedStatus != "skipped_failed_transaction" {
		t.Fatalf("failed status=%q want skipped_failed_transaction", failedStatus)
	}
	if failedAttempts != 0 {
		t.Fatalf("failed WSS transaction consumed an enrichment attempt: %d", failedAttempts)
	}
}
