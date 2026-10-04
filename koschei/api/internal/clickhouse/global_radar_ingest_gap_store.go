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

const globalRadarIngestGapSortingKey = "cursor_key, gap_key"

type globalRadarIngestGapRow struct {
	GapKey             string     `json:"gap_key"`
	SchemaVersion      string     `json:"schema_version"`
	CursorKey          string     `json:"cursor_key"`
	NetworkID          string     `json:"network_id"`
	StreamKind         string     `json:"stream_kind"`
	InitialStartHeight uint64     `json:"initial_start_height"`
	NextHeight         uint64     `json:"next_height"`
	EndHeight          uint64     `json:"end_height"`
	Reason             string     `json:"reason"`
	State              string     `json:"state"`
	DetectedAt         time.Time  `json:"detected_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ResolvedAt         *time.Time `json:"resolved_at"`
	GapVersion         uint64     `json:"gap_version"`
}

type globalRadarIngestGapReadRow struct {
	GapKey             string `json:"gap_key"`
	SchemaVersion      string `json:"schema_version"`
	CursorKey          string `json:"cursor_key"`
	NetworkID          string `json:"network_id"`
	StreamKind         string `json:"stream_kind"`
	InitialStartHeight uint64 `json:"initial_start_height"`
	NextHeight         uint64 `json:"next_height"`
	EndHeight          uint64 `json:"end_height"`
	Reason             string `json:"reason"`
	State              string `json:"state"`
	DetectedAtMillis   int64  `json:"detected_at_ms"`
	UpdatedAtMillis    int64  `json:"updated_at_ms"`
	ResolvedAtMillis   *int64 `json:"resolved_at_ms"`
	GapVersion         uint64 `json:"gap_version"`
}

func (c *Client) SaveGlobalRadarIngestGap(ctx context.Context, gap radarcursor.Gap) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	canonical, err := gap.Canonical()
	if err != nil {
		return fmt.Errorf("validate Global Radar ingest gap: %w", err)
	}
	row := globalRadarIngestGapRow{
		GapKey: canonical.GapKey, SchemaVersion: canonical.SchemaVersion, CursorKey: canonical.CursorKey,
		NetworkID: canonical.NetworkID, StreamKind: canonical.StreamKind,
		InitialStartHeight: canonical.InitialStartHeight, NextHeight: canonical.NextHeight, EndHeight: canonical.EndHeight,
		Reason: canonical.Reason, State: canonical.State, DetectedAt: canonical.DetectedAt,
		UpdatedAt: canonical.UpdatedAt, ResolvedAt: canonical.ResolvedAt,
		GapVersion: uint64(time.Now().UTC().UnixNano()),
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(row); err != nil {
		return fmt.Errorf("encode ClickHouse Global Radar ingest gap: %w", err)
	}
	insertURL := *c.endpoint
	query := insertURL.Query()
	query.Set("query", "INSERT INTO global_radar_ingest_gaps FORMAT JSONEachRow")
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "1")
	query.Set("date_time_input_format", "best_effort")
	insertURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, insertURL.String(), &body)
	if err != nil {
		return fmt.Errorf("build ClickHouse Global Radar ingest gap request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse Global Radar ingest gap insert failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse Global Radar ingest gap insert failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func (c *Client) LoadOpenGlobalRadarIngestGap(ctx context.Context, cursorKey string) (radarcursor.Gap, bool, error) {
	if c == nil {
		return radarcursor.Gap{}, false, fmt.Errorf("ClickHouse client is unavailable")
	}
	cursorKey = strings.TrimSpace(cursorKey)
	if cursorKey == "" || len(cursorKey) > 256 {
		return radarcursor.Gap{}, false, fmt.Errorf("Global Radar ingest cursor key is invalid")
	}
	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", "SELECT gap_key,schema_version,cursor_key,network_id,stream_kind,initial_start_height,next_height,end_height,reason,state,toUnixTimestamp64Milli(detected_at) AS detected_at_ms,toUnixTimestamp64Milli(updated_at) AS updated_at_ms,if(isNull(resolved_at),NULL,toUnixTimestamp64Milli(resolved_at)) AS resolved_at_ms,gap_version FROM global_radar_ingest_gaps FINAL WHERE cursor_key={cursor:String} AND state='open' ORDER BY updated_at DESC,gap_version DESC LIMIT 1 FORMAT JSONEachRow")
	params.Set("param_cursor", cursorKey)
	params.Set("max_execution_time", "10")
	params.Set("max_rows_to_read", "1000000")
	params.Set("max_bytes_to_read", "268435456")
	params.Set("timeout_before_checking_execution_speed", "0")
	params.Set("max_result_rows", "1")
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return radarcursor.Gap{}, false, fmt.Errorf("build ClickHouse Global Radar ingest gap read request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return radarcursor.Gap{}, false, fmt.Errorf("ClickHouse Global Radar ingest gap read failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return radarcursor.Gap{}, false, fmt.Errorf("ClickHouse Global Radar ingest gap read failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	limited := &io.LimitedReader{R: resp.Body, N: 64*1024 + 1}
	decoder := json.NewDecoder(limited)
	var row globalRadarIngestGapReadRow
	if err := decoder.Decode(&row); err != nil {
		if err == io.EOF {
			return radarcursor.Gap{}, false, nil
		}
		return radarcursor.Gap{}, false, fmt.Errorf("decode ClickHouse Global Radar ingest gap: %w", err)
	}
	if limited.N <= 0 {
		return radarcursor.Gap{}, false, fmt.Errorf("ClickHouse Global Radar ingest gap response too large")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return radarcursor.Gap{}, false, fmt.Errorf("ClickHouse Global Radar ingest gap returned multiple rows")
	}
	var resolvedAt *time.Time
	if row.ResolvedAtMillis != nil {
		v := time.UnixMilli(*row.ResolvedAtMillis).UTC()
		resolvedAt = &v
	}
	gap, err := (radarcursor.Gap{
		GapKey: row.GapKey, SchemaVersion: row.SchemaVersion, CursorKey: row.CursorKey,
		NetworkID: row.NetworkID, StreamKind: row.StreamKind,
		InitialStartHeight: row.InitialStartHeight, NextHeight: row.NextHeight, EndHeight: row.EndHeight,
		Reason: row.Reason, State: row.State,
		DetectedAt: time.UnixMilli(row.DetectedAtMillis).UTC(), UpdatedAt: time.UnixMilli(row.UpdatedAtMillis).UTC(),
		ResolvedAt: resolvedAt,
	}).Canonical()
	if err != nil {
		return radarcursor.Gap{}, false, fmt.Errorf("validate stored Global Radar ingest gap: %w", err)
	}
	if gap.CursorKey != cursorKey || row.GapVersion == 0 {
		return radarcursor.Gap{}, false, fmt.Errorf("stored Global Radar ingest gap identity is invalid")
	}
	return gap, true, nil
}

func (c *Client) VerifyGlobalRadarIngestGapSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	metadataURL := *c.endpoint
	params := metadataURL.Query()
	params.Set("query", "SELECT engine, sorting_key FROM system.tables WHERE database={db:String} AND name='global_radar_ingest_gaps' FORMAT JSON")
	params.Set("param_db", c.database)
	params.Set("max_execution_time", "10")
	params.Set("max_rows_to_read", "1000000")
	params.Set("max_bytes_to_read", "268435456")
	params.Set("timeout_before_checking_execution_speed", "0")
	params.Set("max_result_rows", "10")
	metadataURL.RawQuery = params.Encode()
	var metadata tableMetadataResponse
	if err := c.queryJSON(ctx, metadataURL.String(), &metadata); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar ingest gap table metadata: %w", err)
	}
	if len(metadata.Data) != 1 || metadata.Data[0].Engine != "ReplacingMergeTree" || metadata.Data[0].SortingKey != globalRadarIngestGapSortingKey {
		return fmt.Errorf("ClickHouse Global Radar ingest gap table metadata is invalid")
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", "SELECT count() AS count FROM system.columns WHERE database={db:String} AND table='global_radar_ingest_gaps' AND name IN ('gap_key','schema_version','cursor_key','network_id','stream_kind','initial_start_height','next_height','end_height','reason','state','detected_at','updated_at','resolved_at','gap_version','recorded_at') FORMAT JSON")
	columnParams.Set("param_db", c.database)
	columnParams.Set("max_execution_time", "10")
	columnParams.Set("max_result_rows", "10")
	columnsURL.RawQuery = columnParams.Encode()
	var columns schemaCountResponse
	if err := c.queryJSON(ctx, columnsURL.String(), &columns); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar ingest gap columns: %w", err)
	}
	if len(columns.Data) != 1 || columns.Data[0].Count != 15 {
		return fmt.Errorf("ClickHouse Global Radar ingest gap required column contract matched %d of 15 columns", firstSchemaCount(columns))
	}
	return nil
}
