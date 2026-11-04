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

// A PostgreSQL archive row with a syntactically valid but incorrect digest
// must never be exported or marked as externally verified. Only synthetic
// run and staging rows are inserted into the isolated CI database.
func TestArchiveStagingChecksumMustMatchPayloadPostgres17(t *testing.T) {
	databaseURL := testDatabaseURL()
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	var retentionRunID, sourceID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO radar_retention_runs(id,cutoff,status,detail)
		VALUES (gen_random_uuid(),now()-interval '90 days','halted','{"ci_test":"archive_checksum_parity"}'::jsonb)
		RETURNING id::text
	`).Scan(&retentionRunID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := db.ExecContext(cleanCtx, `DELETE FROM radar_retention_archive WHERE run_id=$1::uuid`, retentionRunID); err != nil {
			t.Logf("cleanup staging row: %v", err)
		}
		if _, err := db.ExecContext(cleanCtx, `DELETE FROM radar_retention_runs WHERE id=$1::uuid`, retentionRunID); err != nil {
			t.Logf("cleanup synthetic run: %v", err)
		}
	}()
	if err := db.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	var archiveID int64
	const badChecksum = "0000000000000000000000000000000000000000000000000000000000000000"
	if err := db.QueryRowContext(ctx, `
		INSERT INTO radar_retention_archive
		(run_id,source_table,source_id,row_checksum,payload,archived_at)
		VALUES ($1::uuid,'security_radar_seen_signatures',$2,$3,
		        '{"ci_checksum_mismatch":true}'::jsonb,'2000-01-01'::timestamptz)
		RETURNING id
	`, retentionRunID, sourceID, badChecksum).Scan(&archiveID); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLRepository(db)
	if rows, err := repo.LoadPending(ctx, 5000); err == nil ||
		!strings.Contains(err.Error(), "pending retention archive checksum mismatch") {
		t.Fatalf("corrupt pending archive was returned for export: rows=%d err=%v", len(rows), err)
	}

	exportRunID, err := repo.StartRun(ctx, "s3")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := db.ExecContext(cleanCtx, `DELETE FROM radar_retention_export_runs WHERE id=$1::uuid`, exportRunID); err != nil {
			t.Logf("cleanup synthetic export run: %v", err)
		}
	}()
	staged := ArchiveRow{ID: archiveID, RowChecksum: badChecksum}
	if err := repo.MarkExported(ctx, exportRunID, []ArchiveRow{staged}, "s3://ci-bucket/forbidden#sha256="+badChecksum, badChecksum); err == nil ||
		!strings.Contains(err.Error(), "atomic export mark expected 1 rows, updated 0") {
		t.Fatalf("corrupt staging row was marked exported: %v", err)
	}
	var exportedAt, exportRef sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT exported_at::text,export_ref
		FROM radar_retention_archive WHERE id=$1
	`, archiveID).Scan(&exportedAt, &exportRef); err != nil {
		t.Fatal(err)
	}
	if exportedAt.Valid || exportRef.Valid {
		t.Fatalf("invalid checksum staged row has export marker: at=%v ref=%v", exportedAt.Valid, exportRef.Valid)
	}
}

func testDatabaseURL() string {
	return strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
}
