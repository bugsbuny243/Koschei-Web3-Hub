package retentionexport

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ArchiveLookupFilter only narrows a single approved, already-downloaded
// archive object. ArchivedFrom is inclusive; ArchivedUntil is exclusive.
// These are archive timestamps, not underlying blockchain event timestamps.
type ArchiveLookupFilter struct {
	SourceTable   string
	SourceID      string
	ArchivedFrom  time.Time
	ArchivedUntil time.Time
	Limit         int
}

// ArchiveEvidenceMatch deliberately excludes raw payloads and credentials.
type ArchiveEvidenceMatch struct {
	ArchiveID   int64     `json:"archive_id"`
	SourceTable string    `json:"source_table"`
	SourceID    string    `json:"source_id"`
	RowChecksum string    `json:"row_checksum"`
	ArchivedAt  time.Time `json:"archived_at"`
}

type ArchiveLookupResult struct {
	ObjectSHA256  string                 `json:"object_sha256"`
	VerifiedRows  int64                  `json:"verified_rows"`
	Matches       []ArchiveEvidenceMatch `json:"matches"`
	Truncated     bool                   `json:"truncated"`
}

// LookupArchiveEvidence validates the exact object digest and EVERY row's
// canonical PostgreSQL checksum before returning metadata for any match.
// It never reads production source tables, writes to the database, fetches
// from S3, or exposes row payloads. This is operator-side one-object search,
// not an ARVIS runtime search index or proof of provider object provenance.
func LookupArchiveEvidence(ctx context.Context, db *sql.DB, data []byte, expectedObjectSHA256 string, filter ArchiveLookupFilter) (ArchiveLookupResult, error) {
	result := ArchiveLookupResult{Matches: []ArchiveEvidenceMatch{}}
	expectedObjectSHA256 = strings.ToLower(strings.TrimSpace(expectedObjectSHA256))
	if len(expectedObjectSHA256) != 64 {
		return result, fmt.Errorf("archive lookup requires independently supplied 64-character SHA-256")
	}
	if _, err := hex.DecodeString(expectedObjectSHA256); err != nil {
		return result, fmt.Errorf("archive lookup SHA-256 must be hexadecimal")
	}
	result.ObjectSHA256 = sha256Hex(data)
	if result.ObjectSHA256 != expectedObjectSHA256 {
		return ArchiveLookupResult{Matches: []ArchiveEvidenceMatch{}}, fmt.Errorf("archive lookup object SHA-256 mismatch")
	}
	filter.SourceTable = strings.TrimSpace(filter.SourceTable)
	filter.SourceID = strings.TrimSpace(filter.SourceID)
	if _, allowed := typedRestoreTargets[filter.SourceTable]; !allowed {
		return result, fmt.Errorf("archive lookup source table is not a managed retention target")
	}
	if filter.SourceID == "" && filter.ArchivedFrom.IsZero() && filter.ArchivedUntil.IsZero() {
		return result, fmt.Errorf("archive lookup requires a source ID or archive time bound")
	}
	if !filter.ArchivedFrom.IsZero() && !filter.ArchivedUntil.IsZero() &&
		!filter.ArchivedFrom.Before(filter.ArchivedUntil) {
		return result, fmt.Errorf("archive lookup time range is invalid")
	}
	if filter.Limit == 0 {
		filter.Limit = 25
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return result, fmt.Errorf("archive lookup limit must be between 1 and 100")
	}

	// Verify every row, including records that do not match the filter:
	// a malicious object cannot hide corrupted evidence outside the results.
	verification, err := VerifyExportObject(ctx, db, data)
	if err != nil {
		return ArchiveLookupResult{Matches: []ArchiveEvidenceMatch{}}, fmt.Errorf("archive lookup integrity verification failed: %w", err)
	}
	result.VerifiedRows = verification.Rows
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), retentionVerifyMaxLineBytes)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return ArchiveLookupResult{Matches: []ArchiveEvidenceMatch{}}, err
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var row exportedArchiveLine
		if err := json.Unmarshal(line, &row); err != nil {
			return ArchiveLookupResult{Matches: []ArchiveEvidenceMatch{}}, fmt.Errorf("archive lookup decode failed: %w", err)
		}
		if row.SourceTable != filter.SourceTable ||
			(filter.SourceID != "" && row.SourceID != filter.SourceID) ||
			(!filter.ArchivedFrom.IsZero() && row.ArchivedAt.Before(filter.ArchivedFrom)) ||
			(!filter.ArchivedUntil.IsZero() && !row.ArchivedAt.Before(filter.ArchivedUntil)) {
			continue
		}
		if len(result.Matches) >= filter.Limit {
			result.Truncated = true
			continue
		}
		result.Matches = append(result.Matches, ArchiveEvidenceMatch{
			ArchiveID: row.ArchiveID, SourceTable: row.SourceTable, SourceID: row.SourceID,
			RowChecksum: row.RowChecksum, ArchivedAt: row.ArchivedAt.UTC(),
		})
	}
	if err := scanner.Err(); err != nil {
		return ArchiveLookupResult{Matches: []ArchiveEvidenceMatch{}}, fmt.Errorf("archive lookup scan failed: %w", err)
	}
	return result, nil
}
