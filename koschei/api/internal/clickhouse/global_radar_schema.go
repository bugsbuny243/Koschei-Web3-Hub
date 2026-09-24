package clickhouse

import (
	"context"
	"fmt"
)

const globalRadarGraphSortingKey = "record_type, network, record_date, subject_id, record_id, payload_sha256"

func (c *Client) VerifyGlobalRadarGraphSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}

	metadataURL := *c.endpoint
	metadataParams := metadataURL.Query()
	metadataParams.Set("query", \`SELECT engine, sorting_key
FROM system.tables
WHERE database={db:String} AND name='global_radar_graph_records'
FORMAT JSON\`)
	metadataParams.Set("param_db", c.database)
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
	if metadata.Data[0].Engine != "ReplacingMergeTree" {
		return fmt.Errorf("ClickHouse Global Radar table engine=%q want ReplacingMergeTree", metadata.Data[0].Engine)
	}
	if metadata.Data[0].SortingKey != globalRadarGraphSortingKey {
		return fmt.Errorf("ClickHouse Global Radar sorting_key=%q want %q", metadata.Data[0].SortingKey, globalRadarGraphSortingKey)
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", \`SELECT count() AS count
FROM system.columns
WHERE database={db:String}
  AND table='global_radar_graph_records'
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
FORMAT JSON\`)
	columnParams.Set("param_db", c.database)
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
