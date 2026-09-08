package clickhouse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type publicationTableMetadataResponse struct {
	Data []struct {
		Name       string `json:"name"`
		Engine     string `json:"engine"`
		SortingKey string `json:"sorting_key"`
	} `json:"data"`
}

func (c *Client) ApplyTrustedPublicationMigration(ctx context.Context, migrationSQL string) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	if err := validateTrustedPublicationMigration(migrationSQL); err != nil {
		return err
	}

	migrationURL := *c.endpoint
	query := migrationURL.Query()
	query.Set("multiquery", "1")
	migrationURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, migrationURL.String(), strings.NewReader(migrationSQL))
	if err != nil {
		return fmt.Errorf("build ClickHouse publication migration request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse publication migration request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse publication migration failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func (c *Client) VerifyPublicationLedgerSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT name, engine, sorting_key
FROM system.tables
WHERE database={db:String}
  AND name IN ('dossier_bundle_manifests','dossier_publication_transitions')
ORDER BY name
FORMAT JSON`)
	params.Set("param_db", c.database)
	params.Set("max_execution_time", "10")
	params.Set("max_rows_to_read", "10000")
	params.Set("max_result_rows", "10")
	queryURL.RawQuery = params.Encode()

	var response publicationTableMetadataResponse
	if err := c.queryJSON(ctx, queryURL.String(), &response); err != nil {
		return fmt.Errorf("verify ClickHouse publication ledger metadata: %w", err)
	}
	if len(response.Data) != 2 {
		return fmt.Errorf("ClickHouse publication ledger metadata returned %d of 2 tables", len(response.Data))
	}
	expected := map[string]string{
		"dossier_bundle_manifests":        "case_ref, manifest_id, bundle_sha256",
		"dossier_publication_transitions": "case_ref, sequence, transition_id",
	}
	for _, table := range response.Data {
		wantSorting, ok := expected[table.Name]
		if !ok {
			return fmt.Errorf("unexpected ClickHouse publication ledger table %q", table.Name)
		}
		if table.Engine != "MergeTree" {
			return fmt.Errorf("ClickHouse publication table %s engine=%q want MergeTree", table.Name, table.Engine)
		}
		if table.SortingKey != wantSorting {
			return fmt.Errorf("ClickHouse publication table %s sorting_key=%q want %q", table.Name, table.SortingKey, wantSorting)
		}
	}
	return nil
}

func validateTrustedPublicationMigration(migrationSQL string) error {
	trimmed := strings.TrimSpace(migrationSQL)
	if trimmed == "" {
		return fmt.Errorf("ClickHouse publication migration is empty")
	}
	if len(trimmed) > maxTrustedMigrationBytes {
		return fmt.Errorf("ClickHouse publication migration exceeds %d byte safety limit", maxTrustedMigrationBytes)
	}
	allowed := map[string]bool{
		"koschei_web3.dossier_bundle_manifests":        false,
		"koschei_web3.dossier_publication_transitions": false,
	}
	statementCount := 0
	for _, raw := range strings.Split(stripSQLLineComments(trimmed), ";") {
		statement := strings.TrimSpace(raw)
		if statement == "" {
			continue
		}
		statementCount++
		fields := strings.Fields(statement)
		if len(fields) < 6 || strings.ToUpper(fields[0]) != "CREATE" || strings.ToUpper(fields[1]) != "TABLE" || strings.ToUpper(fields[2]) != "IF" || strings.ToUpper(fields[3]) != "NOT" || strings.ToUpper(fields[4]) != "EXISTS" {
			return fmt.Errorf("ClickHouse publication migration contains non-additive statement")
		}
		name := fields[5]
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("ClickHouse publication migration may not create %s", name)
		}
		if allowed[name] {
			return fmt.Errorf("ClickHouse publication migration repeats %s", name)
		}
		allowed[name] = true
	}
	if statementCount != len(allowed) {
		return fmt.Errorf("ClickHouse publication migration must contain exactly %d CREATE TABLE statements", len(allowed))
	}
	for name, seen := range allowed {
		if !seen {
			return fmt.Errorf("ClickHouse publication migration is missing %s", name)
		}
	}
	return nil
}
