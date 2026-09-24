package clickhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
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


const (
	MaxGlobalRadarEventReadRows   = uint64(5000)
	MaxGlobalRadarEventReadWindow = 31 * 24 * time.Hour
	globalRadarEventReadScanRowCap = uint64(10000000)
	globalRadarEventReadBodyLimit  = int64(64 << 20)
)

type GlobalRadarEventReadRequest struct {
	Network   string
	SubjectID string
	Kind      string
	Since     time.Time
	Until     time.Time
	Limit     uint64
}

type globalRadarEventReadRow struct {
	EventSHA256      string   `json:"event_sha256"`
	SchemaVersion    string   `json:"schema_version"`
	Producer         string   `json:"producer"`
	Kind             string   `json:"kind"`
	NetworkID        string   `json:"network_id"`
	SubjectKind      string   `json:"subject_kind"`
	SubjectID        string   `json:"subject_id"`
	EvidenceState    string   `json:"evidence_state"`
	SourceDigests    []string `json:"source_digests"`
	ObservedAtMillis int64    `json:"observed_at_ms"`
	IngestedAtMillis int64    `json:"ingested_at_ms"`
	PayloadJSON      string   `json:"payload_json"`
	PayloadSHA256    string   `json:"payload_sha256"`
	IngestVersion    uint64   `json:"ingest_version"`
}

type GlobalRadarStoredEvent struct {
	Event         radarevent.Event `json:"event"`
	ObservedAt    time.Time        `json:"observed_at"`
	IngestedAt    time.Time        `json:"ingested_at"`
	PayloadSHA256 string           `json:"payload_sha256"`
	IngestVersion uint64           `json:"ingest_version"`
}

func (c *Client) ReadGlobalRadarEvents(ctx context.Context, request GlobalRadarEventReadRequest) ([]GlobalRadarStoredEvent, error) {
	if c == nil {
		return nil, fmt.Errorf("ClickHouse client is unavailable")
	}
	request, err := normalizeGlobalRadarEventReadRequest(request)
	if err != nil {
		return nil, err
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	queryText := `SELECT
    toString(event_sha256) AS event_sha256,
    schema_version,
    producer,
    kind,
    network_id,
    subject_kind,
    subject_id,
    evidence_state,
    source_digests,
    toUnixTimestamp64Milli(observed_at) AS observed_at_ms,
    toUnixTimestamp64Milli(ingested_at) AS ingested_at_ms,
    payload_json,
    toString(payload_sha256) AS payload_sha256,
    ingest_version
FROM global_radar_events FINAL
WHERE event_date >= toDate({since:DateTime64(3)})
  AND event_date <= toDate({until:DateTime64(3)})
  AND observed_at >= {since:DateTime64(3)}
  AND observed_at < {until:DateTime64(3)}
  AND network_id = {network:String}`
	if request.SubjectID != "" {
		queryText += "\n  AND subject_id = {subject:String}"
	}
	if request.Kind != "" {
		queryText += "\n  AND kind = {kind:String}"
	}
	queryText += "\nORDER BY observed_at ASC, kind ASC, subject_id ASC, event_sha256 ASC, payload_sha256 ASC\nLIMIT {limit:UInt64}\nFORMAT JSONEachRow"

	params.Set("query", queryText)
	params.Set("param_since", globalRadarTimeParam(request.Since))
	params.Set("param_until", globalRadarTimeParam(request.Until))
	params.Set("param_network", request.Network)
	if request.SubjectID != "" {
		params.Set("param_subject", request.SubjectID)
	}
	if request.Kind != "" {
		params.Set("param_kind", request.Kind)
	}
	params.Set("param_limit", strconv.FormatUint(request.Limit+1, 10))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(globalRadarEventReadScanRowCap, 10))
	params.Set("max_result_rows", strconv.FormatUint(request.Limit+1, 10))
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build ClickHouse Global Radar event read request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ClickHouse Global Radar event read request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ClickHouse Global Radar event read failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	limited := &io.LimitedReader{R: resp.Body, N: globalRadarEventReadBodyLimit + 1}
	decoder := json.NewDecoder(limited)
	out := make([]GlobalRadarStoredEvent, 0)
	for {
		var row globalRadarEventReadRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode ClickHouse Global Radar event row: %w", err)
		}
		if uint64(len(out)) >= request.Limit {
			return nil, fmt.Errorf("ClickHouse Global Radar event read exceeded row limit %d", request.Limit)
		}
		item, err := validateGlobalRadarEventReadRow(row, request)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("ClickHouse Global Radar event response exceeded %d bytes", globalRadarEventReadBodyLimit)
	}
	return out, nil
}

func normalizeGlobalRadarEventReadRequest(request GlobalRadarEventReadRequest) (GlobalRadarEventReadRequest, error) {
	request.Network = strings.TrimSpace(request.Network)
	request.SubjectID = strings.TrimSpace(request.SubjectID)
	request.Kind = strings.ToLower(strings.TrimSpace(request.Kind))
	request.Since = request.Since.UTC()
	request.Until = request.Until.UTC()
	if request.Network == "" {
		return GlobalRadarEventReadRequest{}, fmt.Errorf("Global Radar event network is required")
	}
	if request.Since.IsZero() || request.Until.IsZero() || !request.Since.Before(request.Until) {
		return GlobalRadarEventReadRequest{}, fmt.Errorf("Global Radar event read window is invalid")
	}
	if request.Until.Sub(request.Since) > MaxGlobalRadarEventReadWindow {
		return GlobalRadarEventReadRequest{}, fmt.Errorf("Global Radar event read window exceeds %s", MaxGlobalRadarEventReadWindow)
	}
	if request.Limit == 0 || request.Limit > MaxGlobalRadarEventReadRows {
		return GlobalRadarEventReadRequest{}, fmt.Errorf("Global Radar event row limit must be between 1 and %d", MaxGlobalRadarEventReadRows)
	}
	return request, nil
}

func validateGlobalRadarEventReadRow(row globalRadarEventReadRow, request GlobalRadarEventReadRequest) (GlobalRadarStoredEvent, error) {
	if !hex64RE.MatchString(strings.TrimSpace(row.EventSHA256)) ||
		!hex64RE.MatchString(strings.TrimSpace(row.PayloadSHA256)) ||
		strings.TrimSpace(row.SchemaVersion) != radarevent.SchemaVersionV1 ||
		strings.TrimSpace(row.Producer) == "" ||
		strings.TrimSpace(row.Kind) == "" ||
		strings.TrimSpace(row.NetworkID) == "" ||
		strings.TrimSpace(row.SubjectKind) == "" ||
		strings.TrimSpace(row.SubjectID) == "" {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row has invalid identity metadata")
	}
	if row.NetworkID != request.Network {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row escaped requested network boundary")
	}
	if request.SubjectID != "" && row.SubjectID != request.SubjectID {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row escaped requested subject boundary")
	}
	if request.Kind != "" && row.Kind != request.Kind {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row escaped requested kind boundary")
	}

	observedAt := time.UnixMilli(row.ObservedAtMillis).UTC()
	if observedAt.Before(request.Since) || !observedAt.Before(request.Until) {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row escaped requested time boundary")
	}
	ingestedAt := time.UnixMilli(row.IngestedAtMillis).UTC()
	if row.IngestedAtMillis <= 0 {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row has invalid ingest time")
	}

	payload := json.RawMessage(row.PayloadJSON)
	if !json.Valid(payload) {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row contains invalid payload JSON")
	}
	if got := jsonPayloadSHA256(payload); got != row.PayloadSHA256 {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row payload hash mismatch")
	}

	var event radarevent.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return GlobalRadarStoredEvent{}, fmt.Errorf("decode stored Global Radar event payload: %w", err)
	}
	if err := event.Verify(); err != nil {
		return GlobalRadarStoredEvent{}, fmt.Errorf("verify stored Global Radar event payload: %w", err)
	}
	if event.EventSHA256 != row.EventSHA256 ||
		event.SchemaVersion != row.SchemaVersion ||
		event.Producer != row.Producer ||
		event.Kind != row.Kind ||
		event.NetworkID != row.NetworkID ||
		event.SubjectKind != row.SubjectKind ||
		event.SubjectID != row.SubjectID ||
		string(event.State) != row.EvidenceState ||
		event.ObservedAtUnixMS != row.ObservedAtMillis ||
		!reflect.DeepEqual(event.SourceDigests, row.SourceDigests) {
		return GlobalRadarStoredEvent{}, fmt.Errorf("ClickHouse Global Radar event row does not match canonical payload identity")
	}

	return GlobalRadarStoredEvent{
		Event:         event,
		ObservedAt:    observedAt,
		IngestedAt:    ingestedAt,
		PayloadSHA256: row.PayloadSHA256,
		IngestVersion: row.IngestVersion,
	}, nil
}
