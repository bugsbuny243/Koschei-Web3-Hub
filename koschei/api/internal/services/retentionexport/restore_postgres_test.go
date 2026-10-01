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

	checksum := func(payload string) string {
		t.Helper()
		var value string
		if err := db.QueryRowContext(ctx, `
			SELECT encode(sha256(convert_to($1::jsonb::text,'UTF8')),'hex')
		`, payload).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	rows := []ArchiveRow{
		{ID: 1, RunID: "run-r", SourceTable: "security_radar_events", SourceID: "event-restore-1", RowChecksum: checksum(`{"id":1,"kind":"event"}`), Payload: []byte(`{"id":1,"kind":"event"}`), ArchivedAt: time.Unix(100, 0).UTC()},
		{ID: 2, RunID: "run-r", SourceTable: "security_radar_verdicts", SourceID: "verdict-restore-1", RowChecksum: checksum(`{"id":2,"kind":"verdict"}`), Payload: []byte(`{"id":2,"kind":"verdict"}`), ArchivedAt: time.Unix(101, 0).UTC()},
	}
	data, err := serializeBatch(rows)
	if err != nil {
		t.Fatal(err)
	}
	expectedObjectSHA := sha256Hex(data)
	result, err := VerifyRestoreAcceptance(ctx, db, data, expectedObjectSHA)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.SourceTables != 2 || result.ChecksumMismatches != 0 || result.ObjectSHA256 != expectedObjectSHA {
		t.Fatalf("unexpected restore acceptance result: %+v", result)
	}

	if _, err := VerifyRestoreAcceptance(ctx, db, data, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "object checksum mismatch") {
		t.Fatalf("wrong object checksum was not rejected: %v", err)
	}

	tampered := []byte(strings.Replace(string(data), `"kind":"event"`, `"kind":"tampered"`, 1))
	if _, err := VerifyRestoreAcceptance(ctx, db, tampered, ""); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered restore object was not rejected: %v", err)
	}
}
