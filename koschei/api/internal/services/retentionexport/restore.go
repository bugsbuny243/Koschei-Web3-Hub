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

type RestoreAcceptanceResult struct {
	Rows               int64  `json:"rows"`
	SourceTables       int64  `json:"source_tables"`
	ChecksumMismatches int64  `json:"checksum_mismatches"`
	ObjectSHA256       string `json:"object_sha256"`
}

func VerifyRestoreAcceptance(ctx context.Context, db *sql.DB, data []byte, expectedObjectSHA256 string) (RestoreAcceptanceResult, error) {
	var result RestoreAcceptanceResult
	if db == nil {
		return result, fmt.Errorf("retention restore verification database is required")
	}
	expectedObjectSHA256 = strings.ToLower(strings.TrimSpace(expectedObjectSHA256))
	result.ObjectSHA256 = sha256Hex(data)
	if expectedObjectSHA256 != "" {
		if len(expectedObjectSHA256) != 64 {
			return result, fmt.Errorf("expected retention object checksum must be sha256")
		}
		if result.ObjectSHA256 != expectedObjectSHA256 {
			return result, fmt.Errorf("retention object checksum mismatch: expected=%s actual=%s", expectedObjectSHA256, result.ObjectSHA256)
		}
	}

	verified, err := VerifyExportObject(ctx, db, data)
	if err != nil {
		return result, fmt.Errorf("verify retention export before staged restore: %w", err)
	}
	result.Rows = verified.Rows
	result.ChecksumMismatches = verified.ChecksumMismatches

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin retention restore acceptance: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE koschei_retention_restore_stage (
			source_table text NOT NULL,
			source_id text NOT NULL,
			row_checksum text NOT NULL,
			payload jsonb NOT NULL,
			PRIMARY KEY (source_table, source_id)
		) ON COMMIT DROP
	`); err != nil {
		return result, fmt.Errorf("create retention restore staging table: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), retentionVerifyMaxLineBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var row exportedArchiveLine
		if err := json.Unmarshal(line, &row); err != nil {
			return result, fmt.Errorf("decode staged retention restore row: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO koschei_retention_restore_stage
				(source_table,source_id,row_checksum,payload)
			VALUES ($1,$2,$3,$4::jsonb)
		`, strings.TrimSpace(row.SourceTable), strings.TrimSpace(row.SourceID), strings.ToLower(strings.TrimSpace(row.RowChecksum)), string(row.Payload)); err != nil {
			return result, fmt.Errorf("stage retention restore row %s/%s: %w", row.SourceTable, row.SourceID, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("scan staged retention restore object: %w", err)
	}

	var stagedRows, sourceTables, mismatches int64
	if err := tx.QueryRowContext(ctx, `
		SELECT
			count(*),
			count(DISTINCT source_table),
			count(*) FILTER (
				WHERE encode(sha256(convert_to(payload::text,'UTF8')),'hex') <> row_checksum
			)
		FROM koschei_retention_restore_stage
	`).Scan(&stagedRows, &sourceTables, &mismatches); err != nil {
		return result, fmt.Errorf("verify staged retention restore parity: %w", err)
	}
	result.SourceTables = sourceTables
	result.ChecksumMismatches += mismatches
	if stagedRows != verified.Rows {
		return result, fmt.Errorf("retention restore row-count mismatch: exported=%d staged=%d", verified.Rows, stagedRows)
	}
	if result.ChecksumMismatches != 0 {
		return result, fmt.Errorf("retention restore checksum parity failed: mismatches=%d", result.ChecksumMismatches)
	}

	// The acceptance transaction is intentionally never committed. Successful
	// reconstruction proves that the exported object can be restored into
	// PostgreSQL canonical jsonb form without mutating production source tables.
	return result, nil
}
