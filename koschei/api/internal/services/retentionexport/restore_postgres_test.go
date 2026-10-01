package retentionexport

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestVerifyRestoreAcceptancePostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	stamp := time.Now().UTC().Format("20060102150405.000000000")
	var firstID, secondID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_radar_seen_signatures
			(signature,module_id,source_address,network,seen_at,source_target,slot,created_at)
		VALUES ($1,'restore-acceptance-ci','restore-source','solana-mainnet',now(),'restore-target',12345,now())
		RETURNING id::text
	`, "restore-acceptance-a-"+stamp).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO security_radar_seen_signatures
			(signature,module_id,source_address,network,seen_at,source_target,slot,created_at)
		VALUES ($1,'restore-acceptance-ci','restore-source','solana-mainnet',now(),'restore-target',12346,now())
		RETURNING id::text
	`, "restore-acceptance-b-"+stamp).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM security_radar_seen_signatures
			WHERE id IN ($1::uuid,$2::uuid)
		`, firstID, secondID)
	}()

	loadArchiveRow := func(archiveID int64, sourceID string) ArchiveRow {
		t.Helper()
		var payload []byte
		var checksum string
		if err := db.QueryRowContext(ctx, `
			SELECT to_jsonb(s)::text,
			       encode(sha256(convert_to(to_jsonb(s)::text,'UTF8')),'hex')
			FROM security_radar_seen_signatures s
			WHERE id=$1::uuid
		`, sourceID).Scan(&payload, &checksum); err != nil {
			t.Fatal(err)
		}
		return ArchiveRow{
			ID: archiveID, RunID: "run-r", SourceTable: "security_radar_seen_signatures",
			SourceID: sourceID, RowChecksum: checksum, Payload: payload, ArchivedAt: time.Now().UTC(),
		}
	}

	data, err := serializeBatch([]ArchiveRow{
		loadArchiveRow(1, firstID),
		loadArchiveRow(2, secondID),
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedObjectSHA := sha256Hex(data)
	result, err := VerifyRestoreAcceptance(ctx, db, data, expectedObjectSHA)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 ||
		result.SourceTables != 1 ||
		result.TypedRows != 2 ||
		result.TypedSourceTables != 1 ||
		result.SourceIDMismatches != 0 ||
		result.ChecksumMismatches != 0 ||
		result.ObjectSHA256 != expectedObjectSHA {
		t.Fatalf("unexpected restore acceptance result: %+v", result)
	}

	if _, err := VerifyRestoreAcceptance(ctx, db, data, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "object checksum mismatch") {
		t.Fatalf("wrong object checksum was not rejected: %v", err)
	}

	wrongID := "00000000-0000-0000-0000-000000000000"
	tamperedIdentity := []byte(strings.Replace(string(data), firstID, wrongID, 1))
	result, err = VerifyRestoreAcceptance(ctx, db, tamperedIdentity, "")
	if err == nil || result.SourceIDMismatches != 1 {
		t.Fatalf("tampered source identity was not rejected: result=%+v err=%v", result, err)
	}
}
