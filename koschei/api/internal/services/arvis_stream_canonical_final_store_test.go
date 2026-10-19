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

func TestArvisStreamCanonicalFinalEnvelopeDoesNotMintSigningAuthority(t *testing.T) {
	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	envelope := arvisStreamCanonicalFinalEnvelope{
		Target: event.Target, Network: event.Network, Digest: verdict.Digest,
		KeyID: verdict.KeyID, PayloadHash: verdict.PayloadHash,
		SignatureAlgorithm: verdict.SignatureAlgorithm,
		Verdict:            arvisStreamRawUnifiedVerdict(verdict),
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var stored arvisStreamCanonicalFinalEnvelope
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	decoded := UnifiedRadarVerdict(stored.Verdict)
	decoded.Target = stored.Target
	decoded.Network = stored.Network
	decoded.Digest = stored.Digest
	decoded.KeyID = stored.KeyID
	decoded.PayloadHash = stored.PayloadHash
	decoded.SignatureAlgorithm = stored.SignatureAlgorithm
	if err := validateArvisStreamCanonicalVerdict(event, decoded, registry); err != nil {
		t.Fatalf("immutable stored decision failed trusted signature readback: %v", err)
	}
	if decoded.Signature != verdict.Signature || decoded.PayloadHash != verdict.PayloadHash {
		t.Fatal("canonical signature or hash changed during serialization")
	}
}

func TestArvisStreamCanonicalFinalPostgresAtomicAcceptance(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not configured")
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
	defer tx.Rollback() // rollback-only fixture; no test source becomes production evidence

	event, verdict, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality)
		VALUES('ci_fixture','test', $1,$2,'test_canonical_final',$3,'token',$4,'fixture_only')
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

	// A fabricated grade, wrong module and missing verified citation are all
	// rejected before inserting a signed row or completing the queue claim.
	tampered := verdict
	tampered.Grade = "A"
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, tampered, registry); err == nil {
		t.Fatal("tampered signed decision was persisted")
	}
	wrongModule := event
	wrongModule.ModuleID = ModuleRaydiumPoolGuardian
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, wrongModule, verdict, registry); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("raw source mismatch accepted: %v", err)
	}

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, event.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "processing" {
		t.Fatalf("rejected canonical decision marked event as %q", status)
	}
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, verdict, registry); err != nil {
		t.Fatalf("verified signed final and processing completion failed: %v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, event.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("verified signed final left status %q, want completed", status)
	}
	// Retry is idempotent and must not duplicate or rewrite a signed conclusion.
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, verdict, registry); err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM arvis_stream_canonical_finals WHERE stream_event_id=$1::uuid`, event.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("signed final row count=%d, want one", count)
	}

	// Independently signed but changed revision cannot overwrite the immutable
	// first conclusion for this source event.
	revised := verdict
	revised.GeneratedAt = revised.GeneratedAt.Add(time.Second)
	revised = FinalizeUnifiedRadarVerdictContractForNetwork(event.Target, event.Network, revised)
	if !revised.Signed || revised.Signature == verdict.Signature {
		t.Fatal("test setup: revised signed decision must be distinct")
	}
	if err := persistArvisStreamCanonicalFinalTx(ctx, tx, event, revised, registry); err == nil || !strings.Contains(err.Error(), "different immutable canonical final") {
		t.Fatalf("conflicting second signed decision accepted: %v", err)
	}
	// PostgreSQL must enforce append-only storage, even for SQL callers that do
	// not know about the Go writer. Savepoint recovers from expected constraint.
	if _, err := tx.ExecContext(ctx, `SAVEPOINT arvis_immutable`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE arvis_stream_canonical_finals SET key_id='forged' WHERE stream_event_id=$1::uuid`, event.ID); err == nil {
		t.Fatal("immutable signed final unexpectedly updated")
	}
	if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT arvis_immutable`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, event.ID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("atomic processing state lost after conflict: status=%q err=%v", status, err)
	}
}
