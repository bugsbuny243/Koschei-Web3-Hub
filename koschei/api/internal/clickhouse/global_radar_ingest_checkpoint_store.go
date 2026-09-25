package clickhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/radarcursor"
)

const globalRadarIngestCheckpointSortingKey = "cursor_key, height, block_hash, source_event_sha256, state"

type globalRadarIngestCheckpointRow struct {
	CursorKey         string    `json:"cursor_key"`
	SchemaVersion     string    `json:"schema_version"`
	NetworkID         string    `json:"network_id"`
	StreamKind        string    `json:"stream_kind"`
	Height            uint64    `json:"height"`
	BlockHash         string    `json:"block_hash"`
	ParentHash        string    `json:"parent_hash"`
	SourceEventSHA256 string    `json:"source_event_sha256"`
	State             string    `json:"state"`
	ObservedAt        time.Time `json:"observed_at"`
	CheckpointVersion uint64    `json:"checkpoint_version"`
}

type globalRadarIngestCheckpointReadRow struct {
	CursorKey         string `json:"cursor_key"`
	SchemaVersion     string `json:"schema_version"`
	NetworkID         string `json:"network_id"`
	StreamKind        string `json:"stream_kind"`
	Height            uint64 `json:"height"`
	BlockHash         string `json:"block_hash"`
	ParentHash        string `json:"parent_hash"`
	SourceEventSHA256 string `json:"source_event_sha256"`
	State             string `json:"state"`
	ObservedAtMillis  int64  `json:"observed_at_ms"`
	CheckpointVersion uint64 `json:"checkpoint_version"`
}

func (c *Client) SaveGlobalRadarIngestCheckpoint(ctx context.Context, checkpoint radarcursor.Checkpoint) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	canonical, err := checkpoint.Canonical()
	if err != nil {
		return fmt.Errorf("validate Global Radar ingest checkpoint: %w", err)
	}
	row := globalRadarIngestCheckpointRow{
		CursorKey: canonical.CursorKey, SchemaVersion: canonical.SchemaVersion,
		NetworkID: canonical.NetworkID, StreamKind: canonical.StreamKind,
		Height: canonical.Height, BlockHash: canonical.BlockHash, ParentHash: canonical.ParentHash,
		SourceEventSHA256: canonical.SourceEventSHA256, State: canonical.State,
		ObservedAt: canonical.ObservedAt, CheckpointVersion: uint64(time.Now().UTC().UnixNano()),
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(row); err != nil {
		return fmt.Errorf("encode ClickHouse Global Radar ingest checkpoint: %w", err)
	}

	insertURL := *c.endpoint
	query := insertURL.Query()
	query.Set("query", "INSERT INTO global_radar_ingest_checkpoints FORMAT JSONEachRow")
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "1")
	query.Set("date_time_input_format", "best_effort")
	insertURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, insertURL.String(), &body)
	if err != nil {
		return fmt.Errorf("build ClickHouse Global Radar ingest checkpoint request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse Global Radar ingest checkpoint insert failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse Global Radar ingest checkpoint insert failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func (c *Client) LoadGlobalRadarIngestCheckpoint(ctx context.Context, cursorKey string) (radarcursor.Checkpoint, bool, error) {
	if c == nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse client is unavailable")
	}
	cursorKey = strings.TrimSpace(cursorKey)
	if cursorKey == "" || len(cursorKey) > 256 {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("Global Radar ingest cursor key is invalid")
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	queryText := "SELECT\n" +
		"    cursor_key, schema_version, network_id, stream_kind, height, block_hash, parent_hash,\n" +
		"    toString(source_event_sha256) AS source_event_sha256, state,\n" +
		"    toUnixTimestamp64Milli(observed_at) AS observed_at_ms, checkpoint_version\n" +
		"FROM global_radar_ingest_checkpoints\n" +
		"WHERE cursor_key = {cursor:String}\n" +
		"ORDER BY checkpoint_version DESC, recorded_at DESC\n" +
		"LIMIT 1\nFORMAT JSONEachRow"
	params.Set("query", queryText)
	params.Set("param_cursor", cursorKey)
	params.Set("max_execution_time", "10")
	params.Set("max_result_rows", "1")
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("build ClickHouse Global Radar ingest checkpoint read request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar ingest checkpoint read failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar ingest checkpoint read failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	limited := &io.LimitedReader{R: resp.Body, N: 64*1024 + 1}
	decoder := json.NewDecoder(limited)
	var row globalRadarIngestCheckpointReadRow
	if err := decoder.Decode(&row); err != nil {
		if err == io.EOF {
			return radarcursor.Checkpoint{}, false, nil
		}
		return radarcursor.Checkpoint{}, false, fmt.Errorf("decode ClickHouse Global Radar ingest checkpoint: %w", err)
	}
	if limited.N <= 0 {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar ingest checkpoint response too large")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar ingest checkpoint returned multiple rows")
	}

	checkpoint, err := (radarcursor.Checkpoint{
		CursorKey: row.CursorKey, SchemaVersion: row.SchemaVersion,
		NetworkID: row.NetworkID, StreamKind: row.StreamKind, Height: row.Height,
		BlockHash: row.BlockHash, ParentHash: row.ParentHash,
		SourceEventSHA256: row.SourceEventSHA256, State: row.State,
		ObservedAt: time.UnixMilli(row.ObservedAtMillis).UTC(),
	}).Canonical()
	if err != nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("validate stored Global Radar ingest checkpoint: %w", err)
	}
	if checkpoint.CursorKey != cursorKey || row.CheckpointVersion == 0 {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("stored Global Radar ingest checkpoint identity is invalid")
	}
	return checkpoint, true, nil
}

func (c *Client) VerifyGlobalRadarIngestCheckpointSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	metadataURL := *c.endpoint
	params := metadataURL.Query()
	params.Set("query", "SELECT engine, sorting_key FROM system.tables WHERE database={db:String} AND name='global_radar_ingest_checkpoints' FORMAT JSON")
	params.Set("param_db", c.database)
	params.Set("max_execution_time", "10")
	params.Set("max_result_rows", "10")
	metadataURL.RawQuery = params.Encode()

	var metadata tableMetadataResponse
	if err := c.queryJSON(ctx, metadataURL.String(), &metadata); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar ingest checkpoint table metadata: %w", err)
	}
	if len(metadata.Data) != 1 {
		return fmt.Errorf("ClickHouse Global Radar ingest checkpoint table metadata returned %d rows", len(metadata.Data))
	}
	if metadata.Data[0].Engine != "ReplacingMergeTree" {
		return fmt.Errorf("ClickHouse Global Radar ingest checkpoint table engine=%q want ReplacingMergeTree", metadata.Data[0].Engine)
	}
	if metadata.Data[0].SortingKey != globalRadarIngestCheckpointSortingKey {
		return fmt.Errorf("ClickHouse Global Radar ingest checkpoint sorting_key=%q want %q", metadata.Data[0].SortingKey, globalRadarIngestCheckpointSortingKey)
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", "SELECT count() AS count FROM system.columns WHERE database={db:String} AND table='global_radar_ingest_checkpoints' AND name IN ('cursor_key','schema_version','network_id','stream_kind','height','block_hash','parent_hash','source_event_sha256','state','observed_at','checkpoint_version','recorded_at') FORMAT JSON")
	columnParams.Set("param_db", c.database)
	columnParams.Set("max_execution_time", "10")
	columnParams.Set("max_result_rows", "10")
	columnsURL.RawQuery = columnParams.Encode()

	var columns schemaCountResponse
	if err := c.queryJSON(ctx, columnsURL.String(), &columns); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar ingest checkpoint columns: %w", err)
	}
	if len(columns.Data) != 1 || columns.Data[0].Count != 12 {
		return fmt.Errorf("ClickHouse Global Radar ingest checkpoint required column contract matched %d of 12 columns", firstSchemaCount(columns))
	}
	return nil
}

func (c *Client) LoadGlobalRadarCanonicalCheckpointAtHeight(ctx context.Context, cursorKey string, height uint64) (radarcursor.Checkpoint, bool, error) {
	if c == nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse client is unavailable")
	}
	cursorKey = strings.TrimSpace(cursorKey)
	if cursorKey == "" || len(cursorKey) > 256 {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("Global Radar ingest cursor key is invalid")
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	queryText := "SELECT\n" +
		"    cursor_key, schema_version, network_id, stream_kind, height, block_hash, parent_hash,\n" +
		"    toString(source_event_sha256) AS source_event_sha256, state,\n" +
		"    toUnixTimestamp64Milli(observed_at) AS observed_at_ms, checkpoint_version\n" +
		"FROM global_radar_ingest_checkpoints\n" +
		"WHERE cursor_key = {cursor:String} AND height = {height:UInt64} AND state = 'canonical'\n" +
		"ORDER BY checkpoint_version DESC, recorded_at DESC\n" +
		"LIMIT 1\nFORMAT JSONEachRow"
	params.Set("query", queryText)
	params.Set("param_cursor", cursorKey)
	params.Set("param_height", fmt.Sprintf("%d", height))
	params.Set("max_execution_time", "10")
	params.Set("max_result_rows", "1")
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("build ClickHouse Global Radar canonical checkpoint read request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar canonical checkpoint read failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar canonical checkpoint read failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	limited := &io.LimitedReader{R: resp.Body, N: 64*1024 + 1}
	decoder := json.NewDecoder(limited)
	var row globalRadarIngestCheckpointReadRow
	if err := decoder.Decode(&row); err != nil {
		if err == io.EOF {
			return radarcursor.Checkpoint{}, false, nil
		}
		return radarcursor.Checkpoint{}, false, fmt.Errorf("decode ClickHouse Global Radar canonical checkpoint: %w", err)
	}
	if limited.N <= 0 {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar canonical checkpoint response too large")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("ClickHouse Global Radar canonical checkpoint returned multiple rows")
	}
	checkpoint, err := (radarcursor.Checkpoint{
		CursorKey: row.CursorKey, SchemaVersion: row.SchemaVersion,
		NetworkID: row.NetworkID, StreamKind: row.StreamKind, Height: row.Height,
		BlockHash: row.BlockHash, ParentHash: row.ParentHash,
		SourceEventSHA256: row.SourceEventSHA256, State: row.State,
		ObservedAt: time.UnixMilli(row.ObservedAtMillis).UTC(),
	}).Canonical()
	if err != nil {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("validate stored Global Radar canonical checkpoint: %w", err)
	}
	if checkpoint.CursorKey != cursorKey || checkpoint.Height != height || checkpoint.State != radarcursor.StateCanonical || row.CheckpointVersion == 0 {
		return radarcursor.Checkpoint{}, false, fmt.Errorf("stored Global Radar canonical checkpoint identity is invalid")
	}
	return checkpoint, true, nil
}
