package clickhouse

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultDatabase     = "koschei_web3"
	StreamSchemaVersion = uint16(1)
	maxParityRows       = uint64(10000000)
)

type Config struct {
	HTTPURL    string
	Database   string
	User       string
	Password   string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type Client struct {
	endpoint   *url.URL
	database   string
	user       string
	password   string
	httpClient *http.Client
}

type StreamEvent struct {
	EventID         string
	Provider        string
	StreamMode      string
	Network         string
	ModuleID        string
	EventType       string
	Target          string
	TargetType      string
	Signature       string
	Slot            uint64
	ProgramID       string
	EvidenceQuality string
	Decoded         json.RawMessage
	RawEvent        json.RawMessage
	CreatedAt       time.Time
}

type streamEventRow struct {
	EventID         string          `json:"event_id"`
	EventKey        string          `json:"event_key"`
	Provider        string          `json:"provider"`
	StreamMode      string          `json:"stream_mode"`
	Network         string          `json:"network"`
	ModuleID        string          `json:"module_id"`
	EventType       string          `json:"event_type"`
	Target          string          `json:"target"`
	TargetType      string          `json:"target_type"`
	Signature       string          `json:"signature"`
	Slot            uint64          `json:"slot"`
	ProgramID       string          `json:"program_id"`
	EvidenceQuality string          `json:"evidence_quality"`
	Decoded         json.RawMessage `json:"decoded"`
	RawEvent        json.RawMessage `json:"raw_event"`
	DecodedSHA256   string          `json:"decoded_sha256"`
	RawEventSHA256  string          `json:"raw_event_sha256"`
	SchemaVersion   uint16          `json:"schema_version"`
	CreatedAt       time.Time       `json:"created_at"`
	IngestVersion   uint64          `json:"ingest_version"`
}

type streamParityRow struct {
	EventID         string `json:"event_id"`
	EventKey        string `json:"event_key"`
	Provider        string `json:"provider"`
	StreamMode      string `json:"stream_mode"`
	Network         string `json:"network"`
	ModuleID        string `json:"module_id"`
	EventType       string `json:"event_type"`
	Target          string `json:"target"`
	TargetType      string `json:"target_type"`
	Signature       string `json:"signature"`
	Slot            uint64 `json:"slot"`
	ProgramID       string `json:"program_id"`
	EvidenceQuality string `json:"evidence_quality"`
	DecodedSHA256   string `json:"decoded_sha256"`
	RawEventSHA256  string `json:"raw_event_sha256"`
	SchemaVersion   uint16 `json:"schema_version"`
	CreatedAtMillis int64  `json:"created_at_ms"`
}

type countResponse struct {
	Data []struct {
		Count uint64 `json:"count"`
	} `json:"data"`
}

type StreamParityAccumulator struct {
	hasher hash.Hash
	count  uint64
}

func NewStreamParityAccumulator() *StreamParityAccumulator {
	return &StreamParityAccumulator{hasher: sha256.New()}
}

func (a *StreamParityAccumulator) Add(event StreamEvent) error {
	if a == nil || a.hasher == nil {
		return fmt.Errorf("stream parity accumulator is unavailable")
	}
	row, err := normalizeStreamEvent(event, 0)
	if err != nil {
		return err
	}
	a.addRow(parityRowFromStreamEventRow(row))
	return nil
}

func (a *StreamParityAccumulator) Count() uint64 {
	if a == nil {
		return 0
	}
	return a.count
}

func (a *StreamParityAccumulator) SumHex() string {
	if a == nil || a.hasher == nil {
		return ""
	}
	return hex.EncodeToString(a.hasher.Sum(nil))
}

func (a *StreamParityAccumulator) addRow(row streamParityRow) {
	if a == nil || a.hasher == nil {
		return
	}
	fields := []string{
		strings.ToLower(strings.TrimSpace(row.EventID)),
		strings.TrimSpace(row.EventKey),
		strings.TrimSpace(row.Provider),
		strings.TrimSpace(row.StreamMode),
		strings.TrimSpace(row.Network),
		strings.TrimSpace(row.ModuleID),
		strings.TrimSpace(row.EventType),
		strings.TrimSpace(row.Target),
		strings.TrimSpace(row.TargetType),
		strings.TrimSpace(row.Signature),
		strconv.FormatUint(row.Slot, 10),
		strings.TrimSpace(row.ProgramID),
		strings.TrimSpace(row.EvidenceQuality),
		strings.TrimSpace(row.DecodedSHA256),
		strings.TrimSpace(row.RawEventSHA256),
		strconv.FormatUint(uint64(row.SchemaVersion), 10),
		strconv.FormatInt(row.CreatedAtMillis, 10),
	}
	writeLengthPrefixedFields(a.hasher, fields)
	a.count++
}

func NewFromEnv() (*Client, error) {
	timeout := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("CLICKHOUSE_TIMEOUT_MS")); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms >= 250 && ms <= 30000 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return New(Config{
		HTTPURL:  strings.TrimSpace(os.Getenv("CLICKHOUSE_HTTP_URL")),
		Database: firstNonEmpty(strings.TrimSpace(os.Getenv("CLICKHOUSE_DATABASE")), DefaultDatabase),
		User:     firstNonEmpty(strings.TrimSpace(os.Getenv("CLICKHOUSE_USER")), "default"),
		Password: os.Getenv("CLICKHOUSE_PASSWORD"),
		Timeout:  timeout,
	})
}

func New(cfg Config) (*Client, error) {
	rawURL := strings.TrimSpace(cfg.HTTPURL)
	if rawURL == "" {
		return nil, fmt.Errorf("CLICKHOUSE_HTTP_URL is required")
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse ClickHouse HTTP URL: %w", err)
	}
	if endpoint.Scheme != "https" {
		return nil, fmt.Errorf("ClickHouse HTTP URL must use https")
	}
	if endpoint.Host == "" {
		return nil, fmt.Errorf("ClickHouse HTTP URL has no host")
	}
	if endpoint.User != nil {
		return nil, fmt.Errorf("ClickHouse credentials must not be embedded in CLICKHOUSE_HTTP_URL")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("CLICKHOUSE_PASSWORD is required")
	}

	database := firstNonEmpty(strings.TrimSpace(cfg.Database), DefaultDatabase)
	user := firstNonEmpty(strings.TrimSpace(cfg.User), "default")
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	copyURL := *endpoint
	copyURL.RawQuery = ""
	copyURL.Fragment = ""
	return &Client{
		endpoint:   &copyURL,
		database:   database,
		user:       user,
		password:   cfg.Password,
		httpClient: httpClient,
	}, nil
}

func (c *Client) InsertStreamEvents(ctx context.Context, events []StreamEvent) error {
	if c == nil || len(events) == 0 {
		return nil
	}

	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	ingestVersion := uint64(time.Now().UTC().UnixMilli())
	for _, event := range events {
		row, err := normalizeStreamEvent(event, ingestVersion)
		if err != nil {
			return err
		}
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("encode ClickHouse shadow stream event: %w", err)
		}
	}

	insertURL := *c.endpoint
	query := insertURL.Query()
	query.Set("query", "INSERT INTO security_radar_stream_events FORMAT JSONEachRow")
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "1")
	query.Set("date_time_input_format", "best_effort")
	insertURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, insertURL.String(), &body)
	if err != nil {
		return fmt.Errorf("build ClickHouse shadow insert request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse shadow insert request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}

	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse shadow insert failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func (c *Client) CountDistinctStreamEvents(ctx context.Context, since, until time.Time) (uint64, error) {
	if c == nil {
		return 0, fmt.Errorf("ClickHouse client is unavailable")
	}
	if since.IsZero() || until.IsZero() || !since.Before(until) {
		return 0, fmt.Errorf("invalid ClickHouse parity window")
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT countDistinct(event_id) AS count
FROM security_radar_stream_events
WHERE event_date >= toDate({since:DateTime64(3)})
  AND event_date <= toDate({until:DateTime64(3)})
  AND created_at >= {since:DateTime64(3)}
  AND created_at < {until:DateTime64(3)}
FORMAT JSON`)
	params.Set("param_since", parityTimeParam(since))
	params.Set("param_until", parityTimeParam(until))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(maxParityRows, 10))
	params.Set("max_result_rows", "10")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("build ClickHouse parity request: %w", err)
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("ClickHouse parity request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("ClickHouse parity query failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	var result countResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode ClickHouse parity response: %w", err)
	}
	if len(result.Data) != 1 {
		return 0, fmt.Errorf("ClickHouse parity response returned %d rows", len(result.Data))
	}
	return result.Data[0].Count, nil
}

func (c *Client) StreamEventsFingerprint(ctx context.Context, since, until time.Time, rowLimit uint64) (string, uint64, error) {
	if c == nil {
		return "", 0, fmt.Errorf("ClickHouse client is unavailable")
	}
	if since.IsZero() || until.IsZero() || !since.Before(until) {
		return "", 0, fmt.Errorf("invalid ClickHouse parity window")
	}
	if rowLimit == 0 || rowLimit > maxParityRows {
		return "", 0, fmt.Errorf("ClickHouse parity row limit must be between 1 and %d", maxParityRows)
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT
    toString(event_id) AS event_id,
    toString(event_key) AS event_key,
    provider,
    stream_mode,
    network,
    module_id,
    event_type,
    target,
    target_type,
    signature,
    slot,
    program_id,
    evidence_quality,
    toString(decoded_sha256) AS decoded_sha256,
    toString(raw_event_sha256) AS raw_event_sha256,
    schema_version,
    toUnixTimestamp64Milli(created_at) AS created_at_ms
FROM security_radar_stream_events FINAL
WHERE event_date >= toDate({since:DateTime64(3)})
  AND event_date <= toDate({until:DateTime64(3)})
  AND created_at >= {since:DateTime64(3)}
  AND created_at < {until:DateTime64(3)}
ORDER BY toString(event_id) ASC
LIMIT {limit:UInt64}
FORMAT JSONEachRow`)
	params.Set("param_since", parityTimeParam(since))
	params.Set("param_until", parityTimeParam(until))
	params.Set("param_limit", strconv.FormatUint(rowLimit+1, 10))
	params.Set("max_execution_time", "60")
	params.Set("max_rows_to_read", strconv.FormatUint(maxParityRows, 10))
	params.Set("max_result_rows", strconv.FormatUint(rowLimit+1, 10))
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return "", 0, fmt.Errorf("build ClickHouse content parity request: %w", err)
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("ClickHouse content parity request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", 0, fmt.Errorf("ClickHouse content parity query failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	accumulator := NewStreamParityAccumulator()
	decoder := json.NewDecoder(resp.Body)
	for {
		var row streamParityRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", accumulator.Count(), fmt.Errorf("decode ClickHouse content parity row: %w", err)
		}
		if accumulator.Count() >= rowLimit {
			return "", accumulator.Count(), fmt.Errorf("ClickHouse content parity exceeded row limit %d", rowLimit)
		}
		accumulator.addRow(row)
	}
	return accumulator.SumHex(), accumulator.Count(), nil
}

// StreamEventKey follows the current PostgreSQL signed-event dedupe boundary.
// Chain-native identifiers remain byte-exact and are never lowercased. Unsigned
// observations include the committed PostgreSQL UUID so distinct rows do not
// collapse into one fingerprint.
func StreamEventKey(event StreamEvent) string {
	fields := []string{
		strings.TrimSpace(event.Signature),
		strings.TrimSpace(event.ProgramID),
		strings.TrimSpace(event.ModuleID),
		strings.TrimSpace(event.EventType),
		strings.TrimSpace(event.Target),
	}
	if fields[0] == "" {
		fields = append(fields, strings.TrimSpace(event.EventID))
	}

	h := sha256.New()
	writeLengthPrefixedFields(h, fields)
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeStreamEvent(event StreamEvent, ingestVersion uint64) (streamEventRow, error) {
	if strings.TrimSpace(event.EventID) == "" {
		return streamEventRow{}, fmt.Errorf("ClickHouse shadow stream event has no event id")
	}
	if event.CreatedAt.IsZero() {
		return streamEventRow{}, fmt.Errorf("ClickHouse shadow stream event %s has no created_at", strings.TrimSpace(event.EventID))
	}
	decoded := validJSONOrEmpty(event.Decoded)
	rawEvent := validJSONOrEmpty(event.RawEvent)
	return streamEventRow{
		EventID:         strings.TrimSpace(event.EventID),
		EventKey:        StreamEventKey(event),
		Provider:        firstNonEmpty(strings.TrimSpace(event.Provider), "unknown"),
		StreamMode:      firstNonEmpty(strings.TrimSpace(event.StreamMode), "unknown"),
		Network:         firstNonEmpty(strings.TrimSpace(event.Network), "unknown"),
		ModuleID:        firstNonEmpty(strings.TrimSpace(event.ModuleID), "unknown"),
		EventType:       firstNonEmpty(strings.TrimSpace(event.EventType), "stream_event"),
		Target:          strings.TrimSpace(event.Target),
		TargetType:      firstNonEmpty(strings.TrimSpace(event.TargetType), "unknown"),
		Signature:       strings.TrimSpace(event.Signature),
		Slot:            event.Slot,
		ProgramID:       strings.TrimSpace(event.ProgramID),
		EvidenceQuality: firstNonEmpty(strings.TrimSpace(event.EvidenceQuality), "raw_stream"),
		Decoded:         decoded,
		RawEvent:        rawEvent,
		DecodedSHA256:   jsonPayloadSHA256(decoded),
		RawEventSHA256:  jsonPayloadSHA256(rawEvent),
		SchemaVersion:   StreamSchemaVersion,
		CreatedAt:       event.CreatedAt.UTC().Truncate(time.Millisecond),
		IngestVersion:   ingestVersion,
	}, nil
}

func parityRowFromStreamEventRow(row streamEventRow) streamParityRow {
	return streamParityRow{
		EventID:         row.EventID,
		EventKey:        row.EventKey,
		Provider:        row.Provider,
		StreamMode:      row.StreamMode,
		Network:         row.Network,
		ModuleID:        row.ModuleID,
		EventType:       row.EventType,
		Target:          row.Target,
		TargetType:      row.TargetType,
		Signature:       row.Signature,
		Slot:            row.Slot,
		ProgramID:       row.ProgramID,
		EvidenceQuality: row.EvidenceQuality,
		DecodedSHA256:   row.DecodedSHA256,
		RawEventSHA256:  row.RawEventSHA256,
		SchemaVersion:   row.SchemaVersion,
		CreatedAtMillis: row.CreatedAt.UnixMilli(),
	}
}

func jsonPayloadSHA256(value json.RawMessage) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func writeLengthPrefixedFields(destination io.Writer, fields []string) {
	for _, field := range fields {
		_, _ = io.WriteString(destination, strconv.Itoa(len(field)))
		_, _ = io.WriteString(destination, ":")
		_, _ = io.WriteString(destination, field)
		_, _ = io.WriteString(destination, "|")
	}
}

func parityTimeParam(value time.Time) string {
	return value.UTC().Truncate(time.Millisecond).Format("2006-01-02 15:04:05.000")
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("X-ClickHouse-Database", c.database)
	req.Header.Set("X-ClickHouse-User", c.user)
	req.Header.Set("X-ClickHouse-Key", c.password)
}

func validJSONOrEmpty(value json.RawMessage) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(`{}`)
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
