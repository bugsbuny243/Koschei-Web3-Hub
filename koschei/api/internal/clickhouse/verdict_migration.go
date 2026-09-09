package clickhouse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) ApplyTrustedVerdictShadowMigration(ctx context.Context, migrationSQL string) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	if err := validateTrustedVerdictShadowMigration(migrationSQL); err != nil {
		return err
	}
	migrationURL := *c.endpoint
	query := migrationURL.Query()
	query.Set("multiquery", "1")
	migrationURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, migrationURL.String(), strings.NewReader(migrationSQL))
	if err != nil {
		return fmt.Errorf("build ClickHouse verdict migration request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse verdict migration request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse verdict migration failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func (c *Client) VerifyVerdictShadowSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	metadataURL := *c.endpoint
	params := metadataURL.Query()
	params.Set("query", `SELECT engine, sorting_key
FROM system.tables
WHERE database={db:String} AND name='arvis_verdict_snapshots'
FORMAT JSON`)
	params.Set("param_db", c.database)
	params.Set("max_execution_time", "10")
	params.Set("max_result_rows", "10")
	metadataURL.RawQuery = params.Encode()
	var metadata tableMetadataResponse
	if err := c.queryJSON(ctx, metadataURL.String(), &metadata); err != nil {
		return fmt.Errorf("verify ClickHouse verdict table metadata: %w", err)
	}
	if len(metadata.Data) != 1 {
		return fmt.Errorf("ClickHouse verdict table metadata returned %d rows", len(metadata.Data))
	}
	if metadata.Data[0].Engine != "ReplacingMergeTree" {
		return fmt.Errorf("ClickHouse verdict table engine=%q want ReplacingMergeTree", metadata.Data[0].Engine)
	}
	if metadata.Data[0].SortingKey != "module_id, verdict_id" {
		return fmt.Errorf("ClickHouse verdict table sorting_key=%q want %q", metadata.Data[0].SortingKey, "module_id, verdict_id")
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", `SELECT count() AS count
FROM system.columns
WHERE database={db:String}
  AND table='arvis_verdict_snapshots'
  AND (
       (name='verdict_id' AND type='UUID')
    OR (name='event_id' AND type='UUID')
    OR (name='risk_index' AND type='Int32')
    OR (name='evidence' AND type='Array(String)')
    OR (name='evidence_sha256' AND type='FixedString(64)')
    OR (name='signals' AND type='JSON')
    OR (name='signals_sha256' AND type='FixedString(64)')
    OR (name='created_at' AND type='DateTime64(6, \'UTC\')')
    OR (name='source_updated_at' AND type='DateTime64(6, \'UTC\')')
    OR (name='source_version' AND type='UInt64')
  )
FORMAT JSON`)
	columnParams.Set("param_db", c.database)
	columnParams.Set("max_execution_time", "10")
	columnParams.Set("max_result_rows", "10")
	columnsURL.RawQuery = columnParams.Encode()
	var columns schemaCountResponse
	if err := c.queryJSON(ctx, columnsURL.String(), &columns); err != nil {
		return fmt.Errorf("verify ClickHouse verdict table columns: %w", err)
	}
	if len(columns.Data) != 1 || columns.Data[0].Count != 10 {
		return fmt.Errorf("ClickHouse verdict table required column contract matched %d of 10 columns", firstSchemaCount(columns))
	}
	return nil
}

func validateTrustedVerdictShadowMigration(migrationSQL string) error {
	trimmed := strings.TrimSpace(migrationSQL)
	if trimmed == "" {
		return fmt.Errorf("ClickHouse verdict migration is empty")
	}
	if len(trimmed) > maxTrustedMigrationBytes {
		return fmt.Errorf("ClickHouse verdict migration exceeds %d byte safety limit", maxTrustedMigrationBytes)
	}
	statements := strings.Split(stripSQLLineComments(trimmed), ";")
	seen := false
	for _, raw := range statements {
		statement := strings.TrimSpace(raw)
		if statement == "" {
			continue
		}
		if seen {
			return fmt.Errorf("ClickHouse verdict migration must contain exactly one CREATE TABLE statement")
		}
		fields := strings.Fields(statement)
		if len(fields) < 6 || strings.ToUpper(fields[0]) != "CREATE" || strings.ToUpper(fields[1]) != "TABLE" || strings.ToUpper(fields[2]) != "IF" || strings.ToUpper(fields[3]) != "NOT" || strings.ToUpper(fields[4]) != "EXISTS" {
			return fmt.Errorf("ClickHouse verdict migration contains non-additive statement")
		}
		if fields[5] != "koschei_web3.arvis_verdict_snapshots" {
			return fmt.Errorf("ClickHouse verdict migration may only create koschei_web3.arvis_verdict_snapshots")
		}
		seen = true
	}
	if !seen {
		return fmt.Errorf("ClickHouse verdict migration is missing arvis_verdict_snapshots CREATE TABLE")
	}
	return nil
}
