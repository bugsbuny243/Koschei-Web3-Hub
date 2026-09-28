package clickhouse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxTrustedMigrationBytes = 64 << 10

type schemaCountResponse struct {
	Data []struct {
		Count uint64 `json:"count"`
	} `json:"data"`
}

type tableMetadataResponse struct {
	Data []struct {
		Engine     string `json:"engine"`
		SortingKey string `json:"sorting_key"`
	} `json:"data"`
}

// VerifyTrustedMigrationSHA256 pins schema application to the reviewed migration
// bytes. This protects the apply path from accidental or post-checkout file drift.
func VerifyTrustedMigrationSHA256(migrationSQL, expectedHex string) error {
	expected := strings.ToLower(strings.TrimSpace(expectedHex))
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("ClickHouse migration checksum must be %d hex characters", sha256.Size*2)
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return fmt.Errorf("ClickHouse migration checksum is not valid hex: %w", err)
	}
	actualDigest := sha256.Sum256([]byte(migrationSQL))
	actual := hex.EncodeToString(actualDigest[:])
	if actual != expected {
		return fmt.Errorf("ClickHouse migration checksum mismatch: expected=%s actual=%s", expected, actual)
	}
	return nil
}

// ApplyTrustedMigration executes only the narrow additive ClickHouse DDL used by
// the Koschei shadow journal. It intentionally refuses mutation/destructive SQL.
//
// ClickHouse Cloud 26.6 does not accept the legacy "multiquery" HTTP setting.
// Execute each already-validated additive statement as its own HTTPS request
// instead. The migration uses only CREATE ... IF NOT EXISTS, so a partial retry
// remains idempotent.
func (c *Client) ApplyTrustedMigration(ctx context.Context, migrationSQL string) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	if err := validateTrustedMigration(migrationSQL); err != nil {
		return err
	}

	statements := trustedMigrationStatements(migrationSQL)
	for index, statement := range statements {
		if err := c.applyTrustedMigrationStatement(ctx, statement); err != nil {
			return fmt.Errorf("ClickHouse migration statement %d/%d failed: %w", index+1, len(statements), err)
		}
	}
	return nil
}

func (c *Client) applyTrustedMigrationStatement(ctx context.Context, statement string) error {
	migrationURL := *c.endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, migrationURL.String(), strings.NewReader(statement))
	if err != nil {
		return fmt.Errorf("build ClickHouse migration request: %w", err)
	}
	c.applyHeaders(req)
	// CREATE DATABASE must not depend on the target database already existing.
	// Omitting the database header lets ClickHouse execute it in the default
	// database context while preserving the same authenticated user.
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "CREATE DATABASE ") {
		req.Header.Del("X-ClickHouse-Database")
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse migration request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}

	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

// VerifyStreamEventsSchema verifies the contract required by the shadow copier.
// It does not rely on SHOW CREATE formatting so it is stable across server output
// formatting changes.
func (c *Client) VerifyStreamEventsSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}

	metadataURL := *c.endpoint
	metadataParams := metadataURL.Query()
	metadataParams.Set("query", `SELECT engine, sorting_key
FROM system.tables
WHERE database={db:String} AND name='security_radar_stream_events'
FORMAT JSON`)
	metadataParams.Set("param_db", c.database)
	metadataParams.Set("max_execution_time", "10")
	metadataParams.Set("max_result_rows", "10")
	metadataURL.RawQuery = metadataParams.Encode()

	var metadata tableMetadataResponse
	if err := c.queryJSON(ctx, metadataURL.String(), &metadata); err != nil {
		return fmt.Errorf("verify ClickHouse stream table metadata: %w", err)
	}
	if len(metadata.Data) != 1 {
		return fmt.Errorf("ClickHouse stream table metadata returned %d rows", len(metadata.Data))
	}
	if metadata.Data[0].Engine != "ReplacingMergeTree" {
		return fmt.Errorf("ClickHouse stream table engine=%q want ReplacingMergeTree", metadata.Data[0].Engine)
	}
	wantSorting := "network, module_id, stream_mode, event_date, event_id"
	if metadata.Data[0].SortingKey != wantSorting {
		return fmt.Errorf("ClickHouse stream table sorting_key=%q want %q", metadata.Data[0].SortingKey, wantSorting)
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", `SELECT count() AS count
FROM system.columns
WHERE database={db:String}
  AND table='security_radar_stream_events'
  AND (
       (name='event_id' AND type='UUID')
    OR (name='event_key' AND type='FixedString(64)')
    OR (name='decoded' AND type='JSON')
    OR (name='raw_event' AND type='JSON')
    OR (name='decoded_sha256' AND type='FixedString(64)')
    OR (name='raw_event_sha256' AND type='FixedString(64)')
    OR (name='ingest_version' AND type='UInt64')
    OR (name='created_at' AND type='DateTime64(3, \'UTC\')')
    OR (name='event_date' AND type='Date')
  )
FORMAT JSON`)
	columnParams.Set("param_db", c.database)
	columnParams.Set("max_execution_time", "10")
	columnParams.Set("max_result_rows", "10")
	columnsURL.RawQuery = columnParams.Encode()

	var columns schemaCountResponse
	if err := c.queryJSON(ctx, columnsURL.String(), &columns); err != nil {
		return fmt.Errorf("verify ClickHouse stream table columns: %w", err)
	}
	if len(columns.Data) != 1 || columns.Data[0].Count != 9 {
		return fmt.Errorf("ClickHouse stream table required column contract matched %d of 9 columns", firstSchemaCount(columns))
	}
	return nil
}

func (c *Client) queryJSON(ctx context.Context, rawURL string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(destination); err != nil {
		return err
	}
	return nil
}

func validateTrustedMigration(migrationSQL string) error {
	trimmed := strings.TrimSpace(migrationSQL)
	if trimmed == "" {
		return fmt.Errorf("ClickHouse migration is empty")
	}
	if len(trimmed) > maxTrustedMigrationBytes {
		return fmt.Errorf("ClickHouse migration exceeds %d byte safety limit", maxTrustedMigrationBytes)
	}

	statements := trustedMigrationStatements(trimmed)
	seenStreamTable := false
	seenStatement := false
	for _, rawStatement := range statements {
		statement := strings.TrimSpace(rawStatement)
		if statement == "" {
			continue
		}
		seenStatement = true
		fields := strings.Fields(statement)
		if len(fields) < 6 {
			return fmt.Errorf("ClickHouse migration contains malformed or non-additive statement")
		}
		verb := strings.ToUpper(fields[0])
		objectType := strings.ToUpper(fields[1])
		if strings.ToUpper(fields[2]) != "IF" || strings.ToUpper(fields[3]) != "NOT" || strings.ToUpper(fields[4]) != "EXISTS" {
			return fmt.Errorf("ClickHouse migration must use IF NOT EXISTS")
		}

		switch {
		case verb == "CREATE" && objectType == "DATABASE":
			if len(fields) != 6 || fields[5] != "koschei_web3" {
				return fmt.Errorf("ClickHouse migration may only create koschei_web3 database")
			}
		case verb == "CREATE" && objectType == "TABLE":
			if fields[5] != "koschei_web3.security_radar_stream_events" {
				return fmt.Errorf("ClickHouse migration may only create security_radar_stream_events")
			}
			seenStreamTable = true
		default:
			return fmt.Errorf("ClickHouse migration contains non-additive statement")
		}
	}
	if !seenStatement || !seenStreamTable {
		return fmt.Errorf("ClickHouse migration is missing security_radar_stream_events CREATE TABLE")
	}
	return nil
}

func trustedMigrationStatements(value string) []string {
	withoutComments := stripSQLLineComments(value)
	rawStatements := strings.Split(withoutComments, ";")
	statements := make([]string, 0, len(rawStatements))
	for _, rawStatement := range rawStatements {
		if statement := strings.TrimSpace(rawStatement); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}

func stripSQLLineComments(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if index := strings.Index(line, "--"); index >= 0 {
			lines[i] = line[:index]
		}
	}
	return strings.Join(lines, "\n")
}

func firstSchemaCount(response schemaCountResponse) uint64 {
	if len(response.Data) == 0 {
		return 0
	}
	return response.Data[0].Count
}
