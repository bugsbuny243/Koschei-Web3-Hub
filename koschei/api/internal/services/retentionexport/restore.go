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
	Rows                    int64  `json:"rows"`
	SourceTables            int64  `json:"source_tables"`
	ChecksumMismatches      int64  `json:"checksum_mismatches"`
	TypedRows               int64  `json:"typed_rows"`
	TypedSourceTables       int64  `json:"typed_source_tables"`
	SourceIDMismatches      int64  `json:"source_id_mismatches"`
	ObjectSHA256            string `json:"object_sha256"`
}

type typedRestoreTarget struct {
	IDColumn string
	Query    string
}

var typedRestoreTargets = map[string]typedRestoreTarget{
	"security_radar_verdicts": {
		IDColumn: "id",
		Query: `
			SELECT COALESCE(to_jsonb(r)->>$2,''),
			       encode(sha256(convert_to(to_jsonb(r)::text,'UTF8')),'hex')
			FROM jsonb_populate_record(NULL::public.security_radar_verdicts,$1::jsonb) AS r
		`,
	},
	"security_radar_events": {
		IDColumn: "id",
		Query: `
			SELECT COALESCE(to_jsonb(r)->>$2,''),
			       encode(sha256(convert_to(to_jsonb(r)::text,'UTF8')),'hex')
			FROM jsonb_populate_record(NULL::public.security_radar_events,$1::jsonb) AS r
		`,
	},
	"security_radar_seen_signatures": {
		IDColumn: "id",
		Query: `
			SELECT COALESCE(to_jsonb(r)->>$2,''),
			       encode(sha256(convert_to(to_jsonb(r)::text,'UTF8')),'hex')
			FROM jsonb_populate_record(NULL::public.security_radar_seen_signatures,$1::jsonb) AS r
		`,
	},
	"arvis_stream_processing": {
		IDColumn: "stream_event_id",
		Query: `
			SELECT COALESCE(to_jsonb(r)->>$2,''),
			       encode(sha256(convert_to(to_jsonb(r)::text,'UTF8')),'hex')
			FROM jsonb_populate_record(NULL::public.arvis_stream_processing,$1::jsonb) AS r
		`,
	},
	"security_radar_stream_events": {
		IDColumn: "id",
		Query: `
			SELECT COALESCE(to_jsonb(r)->>$2,''),
			       encode(sha256(convert_to(to_jsonb(r)::text,'UTF8')),'hex')
			FROM jsonb_populate_record(NULL::public.security_radar_stream_events,$1::jsonb) AS r
		`,
	},
	"token_trade_events": {
		IDColumn: "id",
		Query: `
			SELECT COALESCE(to_jsonb(r)->>$2,''),
			       encode(sha256(convert_to(to_jsonb(r)::text,'UTF8')),'hex')
			FROM jsonb_populate_record(NULL::public.token_trade_events,$1::jsonb) AS r
		`,
	},
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

	typedTables := map[string]struct{}{}
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
		row.SourceTable = strings.TrimSpace(row.SourceTable)
		row.SourceID = strings.TrimSpace(row.SourceID)
		row.RowChecksum = strings.ToLower(strings.TrimSpace(row.RowChecksum))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO koschei_retention_restore_stage
				(source_table,source_id,row_checksum,payload)
			VALUES ($1,$2,$3,$4::jsonb)
		`, row.SourceTable, row.SourceID, row.RowChecksum, string(row.Payload)); err != nil {
			return result, fmt.Errorf("stage retention restore row %s/%s: %w", row.SourceTable, row.SourceID, err)
		}

		target, ok := typedRestoreTargets[row.SourceTable]
		if !ok {
			return result, fmt.Errorf("retention restore source table %q is not a managed retention target", row.SourceTable)
		}
		var restoredSourceID, restoredChecksum string
		if err := tx.QueryRowContext(ctx, target.Query, string(row.Payload), target.IDColumn).
			Scan(&restoredSourceID, &restoredChecksum); err != nil {
			return result, fmt.Errorf("typed retention restore %s/%s: %w", row.SourceTable, row.SourceID, err)
		}
		result.TypedRows++
		typedTables[row.SourceTable] = struct{}{}
		if strings.TrimSpace(restoredSourceID) != row.SourceID {
			result.SourceIDMismatches++
		}
		if !strings.EqualFold(strings.TrimSpace(restoredChecksum), row.RowChecksum) {
			result.ChecksumMismatches++
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("scan staged retention restore object: %w", err)
	}
	result.TypedSourceTables = int64(len(typedTables))

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
	if stagedRows != verified.Rows || result.TypedRows != verified.Rows {
		return result, fmt.Errorf("retention restore row-count mismatch: exported=%d staged=%d typed=%d", verified.Rows, stagedRows, result.TypedRows)
	}
	if result.SourceIDMismatches != 0 {
		return result, fmt.Errorf("retention restore source-id parity failed: mismatches=%d", result.SourceIDMismatches)
	}
	if result.ChecksumMismatches != 0 {
		return result, fmt.Errorf("retention restore checksum parity failed: mismatches=%d", result.ChecksumMismatches)
	}

	// The acceptance transaction is intentionally never committed. Successful
	// reconstruction proves that each managed retention payload can still be
	// interpreted by its current source-table row type without mutating live
	// security evidence.
	return result, nil
}
