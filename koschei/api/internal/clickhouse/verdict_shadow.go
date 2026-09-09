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
	"strconv"
	"strings"
	"time"
)

const VerdictSnapshotSchemaVersion = uint16(1)

type VerdictSnapshot struct {
	VerdictID       string
	EventID         string
	ModuleID        string
	Target          string
	TargetType      string
	Network         string
	Grade           string
	RiskIndex       int32
	RiskLevel       string
	Verdict         string
	Recommendation  string
	Evidence        []string
	Signals         json.RawMessage
	RuleVersion     string
	Signed          bool
	Signature       string
	Source          string
	EventType       string
	Provider        string
	CreatedAt       time.Time
	SourceUpdatedAt time.Time
}

type verdictSnapshotRow struct {
	VerdictID       string          `json:"verdict_id"`
	EventID         string          `json:"event_id"`
	ModuleID        string          `json:"module_id"`
	Target          string          `json:"target"`
	TargetType      string          `json:"target_type"`
	Network         string          `json:"network"`
	Grade           string          `json:"grade"`
	RiskIndex       int32           `json:"risk_index"`
	RiskLevel       string          `json:"risk_level"`
	Verdict         string          `json:"verdict"`
	Recommendation  string          `json:"recommendation"`
	Evidence        []string        `json:"evidence"`
	EvidenceSHA256  string          `json:"evidence_sha256"`
	Signals         json.RawMessage `json:"signals"`
	SignalsSHA256   string          `json:"signals_sha256"`
	RuleVersion     string          `json:"rule_version"`
	Signed          uint8           `json:"signed"`
	Signature       string          `json:"signature"`
	Source          string          `json:"source"`
	EventType       string          `json:"event_type"`
	Provider        string          `json:"provider"`
	CreatedAt       time.Time       `json:"created_at"`
	SourceUpdatedAt time.Time       `json:"source_updated_at"`
	SourceVersion   uint64          `json:"source_version"`
	SchemaVersion   uint16          `json:"schema_version"`
}

type verdictParityRow struct {
	VerdictID             string          `json:"verdict_id"`
	EventID               string          `json:"event_id"`
	ModuleID              string          `json:"module_id"`
	Target                string          `json:"target"`
	TargetType            string          `json:"target_type"`
	Network               string          `json:"network"`
	Grade                 string          `json:"grade"`
	RiskIndex             int32           `json:"risk_index"`
	RiskLevel             string          `json:"risk_level"`
	Verdict               string          `json:"verdict"`
	Recommendation        string          `json:"recommendation"`
	Evidence              []string        `json:"evidence"`
	EvidenceSHA256        string          `json:"evidence_sha256"`
	Signals               json.RawMessage `json:"signals"`
	SignalsSHA256         string          `json:"signals_sha256"`
	RuleVersion           string          `json:"rule_version"`
	Signed                uint8           `json:"signed"`
	Signature             string          `json:"signature"`
	Source                string          `json:"source"`
	EventType             string          `json:"event_type"`
	Provider              string          `json:"provider"`
	CreatedAtMicros       int64           `json:"created_at_us"`
	SourceUpdatedAtMicros int64           `json:"source_updated_at_us"`
	SourceVersion         uint64          `json:"source_version"`
	SchemaVersion         uint16          `json:"schema_version"`
}

type VerdictParityAccumulator struct {
	hasher hash.Hash
	count  uint64
}

func NewVerdictParityAccumulator() *VerdictParityAccumulator {
	return &VerdictParityAccumulator{hasher: sha256.New()}
}

func (a *VerdictParityAccumulator) Add(snapshot VerdictSnapshot) error {
	if a == nil || a.hasher == nil {
		return fmt.Errorf("verdict parity accumulator is unavailable")
	}
	row, err := normalizeVerdictSnapshot(snapshot)
	if err != nil {
		return err
	}
	return a.addRow(parityRowFromVerdictSnapshotRow(row))
}

func (a *VerdictParityAccumulator) Count() uint64 {
	if a == nil {
		return 0
	}
	return a.count
}

func (a *VerdictParityAccumulator) SumHex() string {
	if a == nil || a.hasher == nil {
		return ""
	}
	return hex.EncodeToString(a.hasher.Sum(nil))
}

func (a *VerdictParityAccumulator) addRow(row verdictParityRow) error {
	if a == nil || a.hasher == nil {
		return fmt.Errorf("verdict parity accumulator is unavailable")
	}
	if got := evidenceListSHA256(row.Evidence); got != strings.TrimSpace(row.EvidenceSHA256) {
		return fmt.Errorf("verdict %s evidence hash mismatch", strings.TrimSpace(row.VerdictID))
	}
	canonicalSignals, err := canonicalJSON(row.Signals)
	if err != nil {
		return fmt.Errorf("verdict %s signals are invalid: %w", strings.TrimSpace(row.VerdictID), err)
	}
	if got := jsonPayloadSHA256(canonicalSignals); got != strings.TrimSpace(row.SignalsSHA256) {
		return fmt.Errorf("verdict %s signals hash mismatch", strings.TrimSpace(row.VerdictID))
	}
	fields := []string{
		strings.ToLower(strings.TrimSpace(row.VerdictID)),
		strings.ToLower(strings.TrimSpace(row.EventID)),
		strings.TrimSpace(row.ModuleID),
		strings.TrimSpace(row.Target),
		strings.TrimSpace(row.TargetType),
		strings.TrimSpace(row.Network),
		strings.TrimSpace(row.Grade),
		strconv.FormatInt(int64(row.RiskIndex), 10),
		strings.TrimSpace(row.RiskLevel),
		strings.TrimSpace(row.Verdict),
		row.Recommendation,
		strings.TrimSpace(row.EvidenceSHA256),
		strings.TrimSpace(row.SignalsSHA256),
		strings.TrimSpace(row.RuleVersion),
		strconv.FormatUint(uint64(row.Signed), 10),
		strings.TrimSpace(row.Signature),
		strings.TrimSpace(row.Source),
		strings.TrimSpace(row.EventType),
		strings.TrimSpace(row.Provider),
		strconv.FormatInt(row.CreatedAtMicros, 10),
		strconv.FormatInt(row.SourceUpdatedAtMicros, 10),
		strconv.FormatUint(row.SourceVersion, 10),
		strconv.FormatUint(uint64(row.SchemaVersion), 10),
	}
	writeLengthPrefixedFields(a.hasher, fields)
	a.count++
	return nil
}

func (c *Client) InsertVerdictSnapshots(ctx context.Context, snapshots []VerdictSnapshot) error {
	if c == nil || len(snapshots) == 0 {
		return nil
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, snapshot := range snapshots {
		row, err := normalizeVerdictSnapshot(snapshot)
		if err != nil {
			return err
		}
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("encode ClickHouse verdict snapshot: %w", err)
		}
	}

	insertURL := *c.endpoint
	query := insertURL.Query()
	query.Set("query", "INSERT INTO arvis_verdict_snapshots FORMAT JSONEachRow")
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "1")
	query.Set("date_time_input_format", "best_effort")
	insertURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, insertURL.String(), &body)
	if err != nil {
		return fmt.Errorf("build ClickHouse verdict shadow insert request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse verdict shadow insert request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse verdict shadow insert failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func (c *Client) CountDistinctVerdictSnapshots(ctx context.Context, since, until time.Time) (uint64, error) {
	if c == nil {
		return 0, fmt.Errorf("ClickHouse client is unavailable")
	}
	if since.IsZero() || until.IsZero() || !since.Before(until) {
		return 0, fmt.Errorf("invalid ClickHouse verdict parity window")
	}
	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT countDistinct(verdict_id) AS count
FROM arvis_verdict_snapshots FINAL
WHERE source_updated_at >= {since:DateTime64(6)}
  AND source_updated_at < {until:DateTime64(6)}
FORMAT JSON`)
	params.Set("param_since", parityTimeMicrosParam(since))
	params.Set("param_until", parityTimeMicrosParam(until))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(maxParityRows, 10))
	params.Set("max_result_rows", "10")
	queryURL.RawQuery = params.Encode()
	var response countResponse
	if err := c.queryJSON(ctx, queryURL.String(), &response); err != nil {
		return 0, fmt.Errorf("query ClickHouse verdict count parity: %w", err)
	}
	if len(response.Data) != 1 {
		return 0, fmt.Errorf("ClickHouse verdict count parity returned %d rows", len(response.Data))
	}
	return response.Data[0].Count, nil
}

func (c *Client) VerdictSnapshotsFingerprint(ctx context.Context, since, until time.Time, rowLimit uint64) (string, uint64, error) {
	if c == nil {
		return "", 0, fmt.Errorf("ClickHouse client is unavailable")
	}
	if since.IsZero() || until.IsZero() || !since.Before(until) {
		return "", 0, fmt.Errorf("invalid ClickHouse verdict parity window")
	}
	if rowLimit == 0 || rowLimit > maxParityRows {
		return "", 0, fmt.Errorf("ClickHouse verdict parity row limit must be between 1 and %d", maxParityRows)
	}
	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT
    toString(verdict_id) AS verdict_id,
    toString(event_id) AS event_id,
    module_id,
    target,
    target_type,
    network,
    grade,
    risk_index,
    risk_level,
    verdict,
    recommendation,
    evidence,
    toString(evidence_sha256) AS evidence_sha256,
    signals,
    toString(signals_sha256) AS signals_sha256,
    rule_version,
    signed,
    signature,
    source,
    event_type,
    provider,
    toUnixTimestamp64Micro(created_at) AS created_at_us,
    toUnixTimestamp64Micro(source_updated_at) AS source_updated_at_us,
    source_version,
    schema_version
FROM arvis_verdict_snapshots FINAL
WHERE source_updated_at >= {since:DateTime64(6)}
  AND source_updated_at < {until:DateTime64(6)}
ORDER BY toString(verdict_id) ASC
LIMIT {limit:UInt64}
FORMAT JSONEachRow`)
	params.Set("param_since", parityTimeMicrosParam(since))
	params.Set("param_until", parityTimeMicrosParam(until))
	params.Set("param_limit", strconv.FormatUint(rowLimit+1, 10))
	params.Set("max_execution_time", "60")
	params.Set("max_rows_to_read", strconv.FormatUint(maxParityRows, 10))
	params.Set("max_result_rows", strconv.FormatUint(rowLimit+1, 10))
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return "", 0, fmt.Errorf("build ClickHouse verdict content parity request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("ClickHouse verdict content parity request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", 0, fmt.Errorf("ClickHouse verdict content parity query failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	accumulator := NewVerdictParityAccumulator()
	decoder := json.NewDecoder(resp.Body)
	for {
		var row verdictParityRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", accumulator.Count(), fmt.Errorf("decode ClickHouse verdict parity row: %w", err)
		}
		if accumulator.Count() >= rowLimit {
			return "", accumulator.Count(), fmt.Errorf("ClickHouse verdict content parity exceeded row limit %d", rowLimit)
		}
		if err := accumulator.addRow(row); err != nil {
			return "", accumulator.Count(), err
		}
	}
	return accumulator.SumHex(), accumulator.Count(), nil
}

func normalizeVerdictSnapshot(snapshot VerdictSnapshot) (verdictSnapshotRow, error) {
	verdictID := strings.TrimSpace(snapshot.VerdictID)
	if verdictID == "" {
		return verdictSnapshotRow{}, fmt.Errorf("ClickHouse verdict snapshot has no verdict id")
	}
	if snapshot.CreatedAt.IsZero() || snapshot.SourceUpdatedAt.IsZero() {
		return verdictSnapshotRow{}, fmt.Errorf("ClickHouse verdict snapshot %s has incomplete source timestamps", verdictID)
	}
	moduleID := strings.TrimSpace(snapshot.ModuleID)
	target := strings.TrimSpace(snapshot.Target)
	if moduleID == "" || target == "" {
		return verdictSnapshotRow{}, fmt.Errorf("ClickHouse verdict snapshot %s has incomplete identity", verdictID)
	}
	canonicalSignals, err := canonicalJSON(validJSONOrEmpty(snapshot.Signals))
	if err != nil {
		return verdictSnapshotRow{}, fmt.Errorf("canonicalize ClickHouse verdict snapshot %s signals: %w", verdictID, err)
	}
	evidence := append([]string(nil), snapshot.Evidence...)
	if evidence == nil {
		evidence = []string{}
	}
	eventID := strings.TrimSpace(snapshot.EventID)
	if eventID == "" {
		eventID = "00000000-0000-0000-0000-000000000000"
	}
	updatedAt := snapshot.SourceUpdatedAt.UTC().Truncate(time.Microsecond)
	createdAt := snapshot.CreatedAt.UTC().Truncate(time.Microsecond)
	version := updatedAt.UnixMicro()
	if version < 0 {
		return verdictSnapshotRow{}, fmt.Errorf("ClickHouse verdict snapshot %s has pre-epoch source_updated_at", verdictID)
	}
	signed := uint8(0)
	if snapshot.Signed {
		signed = 1
	}
	return verdictSnapshotRow{
		VerdictID:       verdictID,
		EventID:         eventID,
		ModuleID:        moduleID,
		Target:          target,
		TargetType:      firstNonEmpty(strings.TrimSpace(snapshot.TargetType), "unknown"),
		Network:         firstNonEmpty(strings.TrimSpace(snapshot.Network), "unknown"),
		Grade:           strings.TrimSpace(snapshot.Grade),
		RiskIndex:       snapshot.RiskIndex,
		RiskLevel:       strings.TrimSpace(snapshot.RiskLevel),
		Verdict:         strings.TrimSpace(snapshot.Verdict),
		Recommendation:  snapshot.Recommendation,
		Evidence:        evidence,
		EvidenceSHA256:  evidenceListSHA256(evidence),
		Signals:         canonicalSignals,
		SignalsSHA256:   jsonPayloadSHA256(canonicalSignals),
		RuleVersion:     strings.TrimSpace(snapshot.RuleVersion),
		Signed:          signed,
		Signature:       strings.TrimSpace(snapshot.Signature),
		Source:          firstNonEmpty(strings.TrimSpace(snapshot.Source), "unknown"),
		EventType:       strings.TrimSpace(snapshot.EventType),
		Provider:        strings.TrimSpace(snapshot.Provider),
		CreatedAt:       createdAt,
		SourceUpdatedAt: updatedAt,
		SourceVersion:   uint64(version),
		SchemaVersion:   VerdictSnapshotSchemaVersion,
	}, nil
}

func parityRowFromVerdictSnapshotRow(row verdictSnapshotRow) verdictParityRow {
	return verdictParityRow{
		VerdictID:             row.VerdictID,
		EventID:               row.EventID,
		ModuleID:              row.ModuleID,
		Target:                row.Target,
		TargetType:            row.TargetType,
		Network:               row.Network,
		Grade:                 row.Grade,
		RiskIndex:             row.RiskIndex,
		RiskLevel:             row.RiskLevel,
		Verdict:               row.Verdict,
		Recommendation:        row.Recommendation,
		Evidence:              append([]string(nil), row.Evidence...),
		EvidenceSHA256:        row.EvidenceSHA256,
		Signals:               append(json.RawMessage(nil), row.Signals...),
		SignalsSHA256:         row.SignalsSHA256,
		RuleVersion:           row.RuleVersion,
		Signed:                row.Signed,
		Signature:             row.Signature,
		Source:                row.Source,
		EventType:             row.EventType,
		Provider:              row.Provider,
		CreatedAtMicros:       row.CreatedAt.UnixMicro(),
		SourceUpdatedAtMicros: row.SourceUpdatedAt.UnixMicro(),
		SourceVersion:         row.SourceVersion,
		SchemaVersion:         row.SchemaVersion,
	}
}

func evidenceListSHA256(evidence []string) string {
	encoded, _ := json.Marshal(evidence)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func canonicalJSON(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 || !json.Valid(value) {
		return nil, fmt.Errorf("invalid JSON")
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func parityTimeMicrosParam(value time.Time) string {
	return value.UTC().Truncate(time.Microsecond).Format("2006-01-02 15:04:05.000000")
}
