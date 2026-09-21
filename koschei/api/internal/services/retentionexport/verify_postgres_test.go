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

func TestVerifyExportObjectPostgres17(t *testing.T) {
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
		{ID: 1, RunID: "run-a", SourceTable: "security_radar_events", SourceID: "event-1", RowChecksum: checksum(`{"id":1,"kind":"event"}`), Payload: []byte(`{"id":1,"kind":"event"}`), ArchivedAt: time.Unix(100, 0).UTC()},
		{ID: 2, RunID: "run-a", SourceTable: "security_radar_verdicts", SourceID: "verdict-1", RowChecksum: checksum(`{"id":2,"kind":"verdict"}`), Payload: []byte(`{"id":2,"kind":"verdict"}`), ArchivedAt: time.Unix(101, 0).UTC()},
	}
	data, err := serializeBatch(rows)
	if err != nil {
		t.Fatal(err)
	}
	result, err := VerifyExportObject(ctx, db, data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.ChecksumMismatches != 0 {
		t.Fatalf("unexpected verification result: %+v", result)
	}

	tampered := []byte(strings.Replace(string(data), `"kind":"event"`, `"kind":"tampered"`, 1))
	result, err = VerifyExportObject(ctx, db, tampered)
	if err == nil || result.ChecksumMismatches != 1 {
		t.Fatalf("tampered object was not rejected: result=%+v err=%v", result, err)
	}

	duplicate := append(append([]byte{}, data...), data[:bytesUntilFirstNewline(data)]...)
	if _, err := VerifyExportObject(ctx, db, duplicate); err == nil || !strings.Contains(err.Error(), "duplicates source identity") {
		t.Fatalf("duplicate source identity was not rejected: %v", err)
	}
}

func bytesUntilFirstNewline(data []byte) int {
	for index, value := range data {
		if value == '\n' {
			return index + 1
		}
	}
	return len(data)
}
