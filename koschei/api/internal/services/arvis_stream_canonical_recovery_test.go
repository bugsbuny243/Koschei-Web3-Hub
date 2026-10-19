package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestArvisStreamCanonicalRecoveryRequiresExplicitTrustedPublicKeys(t *testing.T) {
	t.Setenv(arvisStreamTrustedKeysEnv, "")
	if _, configured, err := arvisStreamRecoveryTrustRegistry(); configured || err != nil {
		t.Fatalf("absent public trust registry was accepted: configured=%v err=%v", configured, err)
	}
	for _, bad := range []string{"not-json", "[]", "[{}]"} {
		t.Setenv(arvisStreamTrustedKeysEnv, bad)
		if _, configured, err := arvisStreamRecoveryTrustRegistry(); !configured || err == nil {
			t.Fatalf("invalid trust registry %q accepted: configured=%v err=%v", bad, configured, err)
		}
	}
	_, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	raw, err := json.Marshal([]GlobalRadarTrustedVerdictKey{registry.keys[verdict.KeyID].config})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(arvisStreamTrustedKeysEnv, string(raw))
	loaded, configured, err := arvisStreamRecoveryTrustRegistry()
	if !configured || err != nil {
		t.Fatalf("valid public trust registry refused: %v", err)
	}
	if _, err := loaded.Resolve(verdict.KeyID, verdict.GeneratedAt); err != nil {
		t.Fatalf("trusted key did not resolve: %v", err)
	}
}

func TestArvisStreamCanonicalRecoveryPostgres17(t *testing.T) {
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
	defer tx.Rollback() // all fixtures are ephemeral

	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality)
		VALUES('ci_fixture','test',$1,$2,'test_canonical_recovery',$3,'token',$4,'fixture_only')
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

	// A crash after a verified receipt was persisted (an older multi-step
	// writer) leaves a stale processing lease. The recovery must independently
	// verify the existing signature and source, not issue a new signature.
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, verdict, registry); err != nil {
		t.Fatalf("seed canonical final: %v", err)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE arvis_stream_processing SET status='processing',
		processed_at=NULL,updated_at=now()-interval '10 minutes'
		WHERE stream_event_id=$1::uuid
	`, event.ID)
	if err != nil {
		t.Fatal(err)
	}

	untrusted, _ := NewGlobalRadarTrustedVerdictRegistry(nil)
	if done, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, event.ID, untrusted); done || err == nil {
		t.Fatalf("untrusted signed receipt completed lease: done=%v err=%v", done, err)
	}
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid
	`, event.ID).Scan(&status); err != nil || status != "processing" {
		t.Fatalf("rejected receipt changed lease: status=%q err=%v", status, err)
	}

	// A ledger source mutation cannot substitute a different transaction even
	// when the receipt and its Ed25519 signature themselves remain valid.
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_radar_stream_events SET signature='unrelated-tx-sig'
		WHERE id=$1::uuid
	`, event.ID); err != nil {
		t.Fatal(err)
	}
	if done, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, event.ID, registry); done || err == nil || !strings.Contains(err.Error(), "source identity mismatch") {
		t.Fatalf("mismatched source completed lease: done=%v err=%v", done, err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE security_radar_stream_events SET signature=$2 WHERE id=$1::uuid
	`, event.ID, event.Signature); err != nil {
		t.Fatal(err)
	}

	done, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, event.ID, registry)
	if err != nil || !done {
		t.Fatalf("trusted source-bound final was not recovered: done=%v err=%v", done, err)
	}
	var processedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT status,processed_at FROM arvis_stream_processing WHERE stream_event_id=$1::uuid
	`, event.ID).Scan(&status, &processedAt); err != nil || status != "completed" || !processedAt.Valid {
		t.Fatalf("atomic recovery state incorrect: status=%q processed=%v err=%v", status, processedAt, err)
	}
	if done, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, event.ID, registry); done || err != nil {
		t.Fatalf("replayed completed lease was accepted: done=%v err=%v", done, err)
	}

	// Old/unknown stale leases WITHOUT a canonical signed final must not be
	// marked complete by this reconciliation path.
	var emptyEventID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality)
		VALUES('ci_fixture','test',$1,$2,'test_empty_recovery',$3,'token','unverified-event','fixture_only')
		RETURNING id::text
	`, event.Network, event.ModuleID, event.Target).Scan(&emptyEventID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO arvis_stream_processing(stream_event_id,target,signature,status,attempts,updated_at)
		VALUES($1::uuid,$2,'unverified-event','processing',1,now()-interval '10 minutes')
	`, emptyEventID, event.Target); err != nil {
		t.Fatal(err)
	}
	if done, err := reconcileArvisStreamCanonicalFinalTx(ctx, tx, emptyEventID, registry); done || err != nil {
		t.Fatalf("missing canonical receipt was completed: done=%v err=%v", done, err)
	}
}
