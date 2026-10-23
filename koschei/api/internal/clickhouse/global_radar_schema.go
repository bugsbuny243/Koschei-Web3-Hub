package clickhouse

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const globalRadarGraphSortingKey = "record_type, network, record_date, subject_id, record_id, payload_sha256"
const globalRadarGraphV2SortingKey = "record_type, network, record_date, subject_id, record_id, snapshot_sha256, payload_sha256"

// The legacy graph table remains the default until its historical snapshots are
// explicitly migrated and reconciled by an operator. Never switch on migration
// creation alone: V2 is additive and initially empty.
func globalRadarGraphTableName() string {
	if strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_GRAPH_V2_ENABLED")) == "1" {
		return "global_radar_graph_records_v2"
	}
	return "global_radar_graph_records"
}

func globalRadarGraphExpectedSortingKey() string {
	if globalRadarGraphTableName() == "global_radar_graph_records_v2" {
		return globalRadarGraphV2SortingKey
	}
	return globalRadarGraphSortingKey
}

// Only a hardcoded, environment-selected table identifier is interpolated into
// query templates; callers cannot provide arbitrary SQL table names.
func globalRadarGraphQuery(query string) string {
	return strings.Replace(query, "FROM global_radar_graph_records FINAL", "FROM "+globalRadarGraphTableName()+" FINAL", 1)
}

func (c *Client) VerifyGlobalRadarGraphSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}

	metadataURL := *c.endpoint
	metadataParams := metadataURL.Query()
	metadataParams.Set("query", `SELECT engine, sorting_key
FROM system.tables
WHERE database={db:String} AND name={table:String}
FORMAT JSON`)
	metadataParams.Set("param_db", c.database)
	metadataParams.Set("param_table", globalRadarGraphTableName())
	metadataParams.Set("max_execution_time", "10")
	metadataParams.Set("max_result_rows", "10")
	metadataURL.RawQuery = metadataParams.Encode()

	var metadata tableMetadataResponse
	if err := c.queryJSON(ctx, metadataURL.String(), &metadata); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar table metadata: %w", err)
	}
	if len(metadata.Data) != 1 {
		return fmt.Errorf("ClickHouse Global Radar table metadata returned %d rows", len(metadata.Data))
	}
	if !isReplacingMergeTreeEngine(metadata.Data[0].Engine) {
		return fmt.Errorf("ClickHouse Global Radar table engine=%q want ReplacingMergeTree", metadata.Data[0].Engine)
	}
	if metadata.Data[0].SortingKey != globalRadarGraphExpectedSortingKey() {
		return fmt.Errorf("ClickHouse Global Radar sorting_key=%q want %q", metadata.Data[0].SortingKey, globalRadarGraphExpectedSortingKey())
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", `SELECT count() AS count
FROM system.columns
WHERE database={db:String}
  AND table={table:String}
  AND name IN (
    'record_type',
    'record_id',
    'schema_version',
    'snapshot_sha256',
    'network',
    'secondary_network',
    'subject_id',
    'target_subject_id',
    'record_kind',
    'evidence_status',
    'evidence_refs',
    'observed_at',
    'recorded_at',
    'payload_json',
    'payload_sha256',
    'ingest_version',
    'ingested_at',
    'record_date'
  )
FORMAT JSON`)
	columnParams.Set("param_db", c.database)
	columnParams.Set("param_table", globalRadarGraphTableName())
	columnParams.Set("max_execution_time", "10")
	columnParams.Set("max_result_rows", "10")
	columnsURL.RawQuery = columnParams.Encode()

	var columns schemaCountResponse
	if err := c.queryJSON(ctx, columnsURL.String(), &columns); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar table columns: %w", err)
	}
	if len(columns.Data) != 1 || columns.Data[0].Count != 18 {
		return fmt.Errorf("ClickHouse Global Radar required column contract matched %d of 18 columns", firstSchemaCount(columns))
	}
	return nil
}

// ClickHouse Cloud rewrites reviewed ReplacingMergeTree DDL to its shared variant.
func isReplacingMergeTreeEngine(engine string) bool {
	return engine == "ReplacingMergeTree" || engine == "SharedReplacingMergeTree" || engine == "ReplicatedReplacingMergeTree"
}
