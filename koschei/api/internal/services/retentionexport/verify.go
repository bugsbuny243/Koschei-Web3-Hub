package retentionexport

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const retentionVerifyMaxLineBytes = 32 << 20

type VerifyObjectResult struct {
	Rows               int64 `json:"rows"`
	ChecksumMismatches int64 `json:"checksum_mismatches"`
}

func VerifyExportObject(ctx context.Context, db *sql.DB, data []byte) (VerifyObjectResult, error) {
	var result VerifyObjectResult
	if db == nil {
		return result, fmt.Errorf("retention verification database is required")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return result, fmt.Errorf("retention export object is empty")
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), retentionVerifyMaxLineBytes)
	seen := map[string]struct{}{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var row exportedArchiveLine
		if err := json.Unmarshal(line, &row); err != nil {
			return result, fmt.Errorf("retention export line %d: decode: %w", lineNumber, err)
		}
		row.SourceTable = strings.TrimSpace(row.SourceTable)
		row.SourceID = strings.TrimSpace(row.SourceID)
		row.RowChecksum = strings.ToLower(strings.TrimSpace(row.RowChecksum))
		if row.ArchiveID <= 0 || row.SourceTable == "" || row.SourceID == "" || len(row.Payload) == 0 || !json.Valid(row.Payload) {
			return result, fmt.Errorf("retention export line %d has invalid archive identity or payload", lineNumber)
		}
		if len(row.RowChecksum) != 64 {
			return result, fmt.Errorf("retention export line %d has invalid row checksum", lineNumber)
		}
		identity := row.SourceTable + "\x00" + row.SourceID
		if _, duplicate := seen[identity]; duplicate {
			return result, fmt.Errorf("retention export line %d duplicates source identity %s/%s", lineNumber, row.SourceTable, row.SourceID)
		}
		seen[identity] = struct{}{}

		var canonicalChecksum string
		if err := db.QueryRowContext(ctx, `
			SELECT encode(sha256(convert_to($1::jsonb::text,'UTF8')),'hex')
		`, string(row.Payload)).Scan(&canonicalChecksum); err != nil {
			return result, fmt.Errorf("retention export line %d canonical checksum: %w", lineNumber, err)
		}
		result.Rows++
		if !strings.EqualFold(strings.TrimSpace(canonicalChecksum), row.RowChecksum) {
			result.ChecksumMismatches++
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("retention export scan: %w", err)
	}
	if result.Rows == 0 {
		return result, fmt.Errorf("retention export object contains no archive rows")
	}
	if result.ChecksumMismatches > 0 {
		return result, fmt.Errorf("retention export object failed row checksum verification: mismatches=%d", result.ChecksumMismatches)
	}
	return result, nil
}
