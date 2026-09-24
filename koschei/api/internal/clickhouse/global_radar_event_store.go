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

	"koschei/api/internal/radarevent"
)

const globalRadarEventSortingKey = "network_id, event_date, kind, subject_id, event_sha256"

type globalRadarEventRow struct {
	EventSHA256   string    `json:"event_sha256"`
	SchemaVersion string    `json:"schema_version"`
	Producer      string    `json:"producer"`
	Kind          string    `json:"kind"`
	NetworkID     string    `json:"network_id"`
	SubjectKind   string    `json:"subject_kind"`
	SubjectID     string    `json:"subject_id"`
	EvidenceState string    `json:"evidence_state"`
	SourceDigests []string  `json:"source_digests"`
	ObservedAt    time.Time `json:"observed_at"`
	PayloadJSON   string    `json:"payload_json"`
	PayloadSHA256 string    `json:"payload_sha256"`
	IngestVersion uint64    `json:"ingest_version"`
}

func (c *Client) InsertGlobalRadarEvents(ctx context.Context, events []radarevent.Event) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	if len(events) == 0 {
		return nil
	}

	ingestVersion := uint64(time.Now().UTC().UnixNano())
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, event := range events {
		row, err := globalRadarEventRowFromEvent(event, ingestVersion)
		if err != nil {
			return err
		}
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("encode ClickHouse Global Radar event: %w", err)
		}
	}

	insertURL := *c.endpoint
	query := insertURL.Query()
	query.Set("query", "INSERT INTO global_radar_events FORMAT JSONEachRow")
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "1")
	query.Set("date_time_input_format", "best_effort")
	insertURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, insertURL.String(), &body)
	if err != nil {
		return fmt.Errorf("build ClickHouse Global Radar event insert request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse Global Radar event insert request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse Global Radar event insert failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func globalRadarEventRowFromEvent(event radarevent.Event, ingestVersion uint64) (globalRadarEventRow, error) {
	if err := event.Verify(); err != nil {
		return globalRadarEventRow{}, fmt.Errorf("verify Global Radar event: %w", err)
	}
	sealed, err := event.Seal()
	if err != nil {
		return globalRadarEventRow{}, fmt.Errorf("canonicalize Global Radar event: %w", err)
	}
	if sealed.EventSHA256 != event.EventSHA256 {
		return globalRadarEventRow{}, fmt.Errorf("Global Radar event canonical digest changed during storage normalization")
	}
	payload, err := json.Marshal(sealed)
	if err != nil {
		return globalRadarEventRow{}, fmt.Errorf("encode Global Radar event payload: %w", err)
	}
	if !json.Valid(payload) {
		return globalRadarEventRow{}, fmt.Errorf("Global Radar event payload is invalid JSON")
	}
	observedAt := time.UnixMilli(sealed.ObservedAtUnixMS).UTC()
	return globalRadarEventRow{
		EventSHA256:   sealed.EventSHA256,
		SchemaVersion: sealed.SchemaVersion,
		Producer:      sealed.Producer,
		Kind:          sealed.Kind,
		NetworkID:     sealed.NetworkID,
		SubjectKind:   sealed.SubjectKind,
		SubjectID:     sealed.SubjectID,
		EvidenceState: string(sealed.State),
		SourceDigests: append([]string(nil), sealed.SourceDigests...),
		ObservedAt:    observedAt,
		PayloadJSON:   string(payload),
		PayloadSHA256: jsonPayloadSHA256(payload),
		IngestVersion: ingestVersion,
	}, nil
}

func (c *Client) VerifyGlobalRadarEventSchema(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}

	metadataURL := *c.endpoint
	metadataParams := metadataURL.Query()
	metadataParams.Set("query", `SELECT engine, sorting_key
FROM system.tables
WHERE database={db:String} AND name='global_radar_events'
FORMAT JSON`)
	metadataParams.Set("param_db", c.database)
	metadataParams.Set("max_execution_time", "10")
	metadataParams.Set("max_result_rows", "10")
	metadataURL.RawQuery = metadataParams.Encode()

	var metadata tableMetadataResponse
	if err := c.queryJSON(ctx, metadataURL.String(), &metadata); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar event table metadata: %w", err)
	}
	if len(metadata.Data) != 1 {
		return fmt.Errorf("ClickHouse Global Radar event table metadata returned %d rows", len(metadata.Data))
	}
	if metadata.Data[0].Engine != "ReplacingMergeTree" {
		return fmt.Errorf("ClickHouse Global Radar event table engine=%q want ReplacingMergeTree", metadata.Data[0].Engine)
	}
	if metadata.Data[0].SortingKey != globalRadarEventSortingKey {
		return fmt.Errorf("ClickHouse Global Radar event sorting_key=%q want %q", metadata.Data[0].SortingKey, globalRadarEventSortingKey)
	}

	columnsURL := *c.endpoint
	columnParams := columnsURL.Query()
	columnParams.Set("query", `SELECT count() AS count
FROM system.columns
WHERE database={db:String}
  AND table='global_radar_events'
  AND name IN (
    'event_sha256',
    'schema_version',
    'producer',
    'kind',
    'network_id',
    'subject_kind',
    'subject_id',
    'evidence_state',
    'source_digests',
    'observed_at',
    'payload_json',
    'payload_sha256',
    'ingest_version',
    'ingested_at',
    'event_date'
  )
FORMAT JSON`)
	columnParams.Set("param_db", c.database)
	columnParams.Set("max_execution_time", "10")
	columnParams.Set("max_result_rows", "10")
	columnsURL.RawQuery = columnParams.Encode()

	var columns schemaCountResponse
	if err := c.queryJSON(ctx, columnsURL.String(), &columns); err != nil {
		return fmt.Errorf("verify ClickHouse Global Radar event table columns: %w", err)
	}
	if len(columns.Data) != 1 || columns.Data[0].Count != 15 {
		return fmt.Errorf("ClickHouse Global Radar event required column contract matched %d of 15 columns", firstSchemaCount(columns))
	}
	return nil
}
