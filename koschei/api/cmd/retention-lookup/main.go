// retention-lookup is an operator-only, read-only search over ONE downloaded
// and independently approved archive object. No HTTP route or S3 listing.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services/retentionexport"

	_ "github.com/lib/pq"
)

func main() {
	var filePath, expectedSHA, sourceTable, sourceID, fromRaw, untilRaw string
	var limit int
	flag.StringVar(&filePath, "file", "", "path to one downloaded archive NDJSON object")
	flag.StringVar(&expectedSHA, "sha256", "", "independently verified SHA-256 from approved archive receipt (required)")
	flag.StringVar(&sourceTable, "source-table", "", "exact retention-managed source table (required)")
	flag.StringVar(&sourceID, "source-id", "", "exact original source ID")
	flag.StringVar(&fromRaw, "archived-from", "", "inclusive archived_at bound (RFC3339)")
	flag.StringVar(&untilRaw, "archived-until", "", "exclusive archived_at bound (RFC3339)")
	flag.IntVar(&limit, "limit", 25, "maximum metadata matches (1..100)")
	flag.Parse()

	if strings.TrimSpace(filePath) == "" || strings.TrimSpace(expectedSHA) == "" ||
		strings.TrimSpace(sourceTable) == "" {
		fail("archive lookup requires -file, -sha256 and -source-table")
	}
	if strings.TrimSpace(sourceID) == "" && strings.TrimSpace(fromRaw) == "" && strings.TrimSpace(untilRaw) == "" {
		fail("archive lookup requires -source-id or an archived-at time bound")
	}
	from, err := parseBound(fromRaw)
	if err != nil {
		fail("invalid -archived-from; expected RFC3339 with timezone")
	}
	until, err := parseBound(untilRaw)
	if err != nil {
		fail("invalid -archived-until; expected RFC3339 with timezone")
	}
	path := strings.TrimSpace(filePath)
	stat, err := os.Stat(path)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > 256<<20 {
		fail("archive lookup file must be a readable regular object of at most 256 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fail("archive lookup could not read object")
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = strings.TrimSpace(os.Getenv("APP_DATABASE_URL"))
	}
	if databaseURL == "" {
		fail("archive lookup requires read-only PostgreSQL credentials in DATABASE_URL or APP_DATABASE_URL")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		fail("archive lookup could not initialize PostgreSQL verifier")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fail("archive lookup PostgreSQL verifier is unavailable")
	}
	result, err := retentionexport.LookupArchiveEvidence(ctx, db, data, expectedSHA, retentionexport.ArchiveLookupFilter{
		SourceTable: sourceTable, SourceID: sourceID, ArchivedFrom: from, ArchivedUntil: until, Limit: limit,
	})
	if err != nil {
		fail(err.Error())
	}
	output, _ := json.Marshal(map[string]any{"ok": true, "mode": "archive_evidence_lookup", "result": result})
	fmt.Println(string(output))
}

func parseBound(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func fail(message string) {
	output, _ := json.Marshal(map[string]any{"ok": false, "error": message})
	fmt.Fprintln(os.Stderr, string(output))
	os.Exit(1)
}
