package retentionexport

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

func TestArchiveEvidenceLookupVerifiesWholeObjectPostgres17(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
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

	when := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	makeRow := func(id int64, sourceID string, archived time.Time) ArchiveRow {
		t.Helper()
		payload := json.RawMessage(`{"id":"` + sourceID + `","sensitive_marker":"NEVER_RETURN_RAW_ARCHIVE_PAYLOAD"}`)
		var canonicalChecksum string
		if err := db.QueryRowContext(ctx, `
			SELECT encode(sha256(convert_to($1::jsonb::text,'UTF8')),'hex')
		`, string(payload)).Scan(&canonicalChecksum); err != nil {
			t.Fatal(err)
		}
		return ArchiveRow{
			ID: id, RunID: "test-run", SourceTable: "security_radar_events",
			SourceID: sourceID, RowChecksum: canonicalChecksum,
			Payload: payload, ArchivedAt: archived,
		}
	}
	rows := []ArchiveRow{makeRow(101, "event-a", when), makeRow(102, "event-b", when.Add(48*time.Hour))}
	data, err := serializeBatch(rows)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256Hex(data)
	filter := ArchiveLookupFilter{SourceTable: "security_radar_events", SourceID: "event-a"}
	found, err := LookupArchiveEvidence(ctx, db, data, digest, filter)
	if err != nil {
		t.Fatal(err)
	}
	if found.VerifiedRows != 2 || found.ObjectSHA256 != digest || found.Truncated ||
		len(found.Matches) != 1 || found.Matches[0].SourceID != "event-a" ||
		found.Matches[0].ArchiveID != 101 || !found.Matches[0].ArchivedAt.Equal(when) {
		t.Fatalf("invalid exact identity lookup: %+v", found)
	}
	safeJSON, err := json.Marshal(found)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(safeJSON), "NEVER_RETURN_RAW_ARCHIVE_PAYLOAD") ||
		strings.Contains(string(safeJSON), "sensitive_marker") ||
		strings.Contains(string(safeJSON), "\"payload\"") {
		t.Fatal("lookup response revealed archived evidence payload")
	}

	// Time-bounded search uses ARCHIVED timestamp and never exposes payloads.
	bounded, err := LookupArchiveEvidence(ctx, db, data, digest, ArchiveLookupFilter{
		SourceTable:   "security_radar_events",
		ArchivedFrom:  when.Add(24 * time.Hour),
		ArchivedUntil: when.Add(72 * time.Hour),
	})
	if err != nil || len(bounded.Matches) != 1 || bounded.Matches[0].SourceID != "event-b" {
		t.Fatalf("archive time bound failed: %+v err=%v", bounded, err)
	}
	limited, err := LookupArchiveEvidence(ctx, db, data, digest, ArchiveLookupFilter{
		SourceTable: "security_radar_events", ArchivedFrom: when, Limit: 1,
	})
	if err != nil || !limited.Truncated || len(limited.Matches) != 1 {
		t.Fatalf("bounded lookup failed closed: %+v err=%v", limited, err)
	}
	for _, tc := range []struct {
		name, expectedSHA string
		data              []byte
		filter            ArchiveLookupFilter
		wantError         string
	}{
		{"missing independently supplied digest", "", data, filter, "requires independently supplied"},
		{"nonhex digest", strings.Repeat("z", 64), data, filter, "must be hexadecimal"},
		{"mismatched digest", strings.Repeat("0", 64), data, filter, "object SHA-256 mismatch"},
		{"unmanaged source table", digest, data, ArchiveLookupFilter{SourceTable: "private_unknown", SourceID: "event-a"}, "not a managed"},
		{"unbounded table scan", digest, data, ArchiveLookupFilter{SourceTable: "security_radar_events"}, "requires a source ID or archive time bound"},
		{"unbounded result limit", digest, data, ArchiveLookupFilter{SourceTable: "security_radar_events", SourceID: "event-a", Limit: 101}, "limit must be between"},
		{"inverted time range", digest, data, ArchiveLookupFilter{SourceTable: "security_radar_events", ArchivedFrom: when.Add(time.Hour), ArchivedUntil: when}, "time range is invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LookupArchiveEvidence(ctx, db, tc.data, tc.expectedSHA, tc.filter); err == nil ||
				!strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("archive lookup incorrectly accepted invalid input: %v", err)
			}
		})
	}

	// A nonmatching row with a corrupt canonical SHA invalidates the entire
	// object, even when the desired row itself remains intact.
	corrupt := append([]ArchiveRow(nil), rows...)
	corrupt[1].RowChecksum = strings.Repeat("f", 64)
	corruptedObject, err := serializeBatch(corrupt)
	if err != nil {
		t.Fatal(err)
	}
	verificationSHA := sha256Hex(corruptedObject)
	if matches, err := LookupArchiveEvidence(ctx, db, corruptedObject, verificationSHA, filter); err == nil ||
		!strings.Contains(err.Error(), "integrity verification failed") || len(matches.Matches) != 0 {
		t.Fatalf("nonmatching corrupt archive row leaked a result: %+v err=%v", matches, err)
	}

	// An attacker could preserve the correct canonical payload checksum
	// while changing only the untrusted source_id metadata in the object.
	// Even that unrelated row invalidates the whole lookup response.
	forged := append([]ArchiveRow(nil), rows...)
	forged[1].SourceID = "forged-source-identity"
	forgedObject, err := serializeBatch(forged)
	if err != nil {
		t.Fatal(err)
	}
	if matches, err := LookupArchiveEvidence(ctx, db, forgedObject, sha256Hex(forgedObject), filter); err == nil ||
		!strings.Contains(err.Error(), "source identity parity failed") || len(matches.Matches) != 0 {
		t.Fatalf("forged source identity was treated as verified evidence: %+v err=%v", matches, err)
	}
}
