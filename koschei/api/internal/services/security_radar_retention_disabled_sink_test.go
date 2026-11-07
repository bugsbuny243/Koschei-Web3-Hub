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

// A missing export destination must never authorize source deletion. The
// compatibility (v1) and current v2 retention workers share this preflight.
func TestRetentionExternalArchiveDisabledFailsClosed(t *testing.T) {
	for _, sink := range []string{"disabled", ""} {
		t.Run("sink_"+sink, func(t *testing.T) {
			t.Setenv("KOSCHEI_RADAR_ARCHIVE_EXPORT_SINK", sink)
			worker := &securityRadarRetentionWorker{}
			err := worker.exportPendingArchive(context.Background(), "before_retention_v2")
			if err == nil || !strings.Contains(err.Error(), "durable destination disabled") {
				t.Fatalf("disabled external archive must fail closed, got: %v", err)
			}
		})
	}
}

// The production worker must write a scrubbed halt reason without touching
// expired source rows when the sink is disabled. This uses an isolated CI DB,
// does not call an S3 service, and removes only its synthetic run ledger row.
func TestRetentionV2DisabledArchiveSinkRecordsHaltPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	t.Setenv("KOSCHEI_RADAR_ARCHIVE_EXPORT_SINK", "disabled")
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

	// Unusual cutoff distinguishes this synthetic CI run from real runs.
	const cutoffDays = 319
	lower := time.Now().UTC().AddDate(0, 0, -cutoffDays).Add(-time.Minute)
	upper := time.Now().UTC().AddDate(0, 0, -cutoffDays).Add(time.Minute)
	worker := &securityRadarRetentionWorkerV2{base: &securityRadarRetentionWorker{
		db: db, days: cutoffDays, interval: 12 * time.Hour,
	}}
	worker.runOnce(ctx)

	var runID, status, haltReason string
	var selected, archived, verified, deleted, pruned int64
	err = db.QueryRowContext(ctx, `
		SELECT id::text,status,COALESCE(detail->>'halt_reason',''),
		       selected_rows,archived_rows,verified_rows,deleted_rows,pruned_rows
		FROM radar_retention_runs
		WHERE cutoff BETWEEN $1 AND $2
		  AND detail->>'halt_reason'='archive_export_preflight_failed'
		ORDER BY started_at DESC,id DESC
		LIMIT 1
	`, lower, upper).Scan(&runID, &status, &haltReason, &selected, &archived, &verified, &deleted, &pruned)
	if err != nil {
		t.Fatalf("disabled sink did not persist a fail-closed retention run: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, "DELETE FROM radar_retention_runs WHERE id=$1::uuid", runID); err != nil {
			t.Logf("clean up isolated CI retention halt: %v", err)
		}
	}()
	if status != "halted" || haltReason != "archive_export_preflight_failed" {
		t.Fatalf("unsafe disabled-archive status=%q reason=%q", status, haltReason)
	}
	if selected != 0 || archived != 0 || verified != 0 || deleted != 0 || pruned != 0 {
		t.Fatalf("disabled export processed source rows: selected=%d archived=%d verified=%d deleted=%d pruned=%d",
			selected, archived, verified, deleted, pruned)
	}
}

// The automatic retention worker cannot treat ephemeral disk or unauthenticated
// plain HTTP as an independently retained archive. The exporter utility keeps
// filesystem support for isolated restore fixtures; no provider call occurs.
func TestRetentionDurableExportPreflightRequiresSecureS3(t *testing.T) {
	for _, tc := range []struct {
		name, mode, endpoint, wantError string
	}{
		{"filesystem", "filesystem", "", "requires S3 archive destination"},
		{"plain HTTP", "s3", "http://archive.example.test", "secure HTTPS S3 endpoint"},
		{"inline URL credentials", "s3", "https://testuser:secret@archive.example.test", "secure HTTPS S3 endpoint"},
		{"invalid endpoint", "s3", "https://", "secure HTTPS S3 endpoint"},
		{"bare HTTPS host", "s3", "archive.example.test", ""},
		{"bare HTTPS host and port", "s3", "archive.example.test:443", ""},
		{"empty endpoint", "s3", "", "secure HTTPS S3 endpoint"},
		{"secure HTTPS", "s3", "https://archive.example.test", ""},
		{"uppercase S3 mode", "S3", "https://archive.example.test", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KOSCHEI_RADAR_ARCHIVE_EXPORT_SINK", tc.mode)
			t.Setenv("KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_ENDPOINT", tc.endpoint)
			err := retentionDurableExportPreflight()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("secure archive config shape unexpectedly rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("archive preflight error=%v, want %q", err, tc.wantError)
			}
			if err != nil && strings.Contains(err.Error(), "testuser:secret") {
				t.Fatal("preflight error exposed inline endpoint credentials")
			}
		})
	}
}
