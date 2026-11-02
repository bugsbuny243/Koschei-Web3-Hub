package retentionexport

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Provider error text may contain signed URLs and request credentials.
// Only fixed reason codes may be persisted in the production export ledger.
func TestArchiveExportRunFailureStoresOnlySafeErrorPostgres17(t *testing.T) {
	databaseURL := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
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
	repository := NewSQLRepository(db)
	const marker = "TEST_ONLY_PROVIDER_CREDENTIAL_MARKER"
	for _, tc := range []struct {
		name       string
		runErr     error
		mismatches int64
		status     string
		errorCode  string
	}{
		{name: "provider request failure", runErr: errors.New("S3 PUT response endpoint?token=" + marker), status: "failed", errorCode: "archive_export_failed"},
		{name: "readback checksum failure", runErr: errors.New("provider readback " + marker), mismatches: 1, status: "failed", errorCode: "archive_export_checksum_mismatch"},
		{name: "successful empty export", status: "completed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runID, err := repository.StartRun(ctx, "s3")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
				defer done()
				if _, err := db.ExecContext(cleanupCtx, `DELETE FROM radar_retention_export_runs WHERE id=$1::uuid`, runID); err != nil {
					t.Logf("clean up isolated export run: %v", err)
				}
			}()
			result := Result{Enabled: true, SelectedRows: 1, ChecksumMismatches: tc.mismatches}
			if err := repository.FinishRun(ctx, runID, result, tc.runErr); err != nil {
				t.Fatal(err)
			}
			var status string
			var errorMessage sql.NullString
			var mismatches int64
			if err := db.QueryRowContext(ctx, `
				SELECT status,error_message,checksum_mismatches
				FROM radar_retention_export_runs WHERE id=$1::uuid
			`, runID).Scan(&status, &errorMessage, &mismatches); err != nil {
				t.Fatal(err)
			}
			if status != tc.status || errorMessage.String != tc.errorCode || mismatches != tc.mismatches {
				t.Fatalf("unsafe export run: status=%q code=%q mismatches=%d", status, errorMessage.String, mismatches)
			}
			if strings.Contains(errorMessage.String, marker) || strings.Contains(errorMessage.String, "endpoint") {
				t.Fatal("provider response or credentials leaked into durable export run ledger")
			}
			if tc.runErr == nil && errorMessage.Valid {
				t.Fatal("successful export run should not persist an error message")
			}
		})
	}
}
