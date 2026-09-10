package clickhouse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	MaxARVISMemoryReadRows   = uint64(2000)
	MaxARVISMemoryReadWindow = 31 * 24 * time.Hour
	arvisMemoryScanRowCap    = uint64(5000000)
	arvisMemoryBodyLimit     = int64(32 << 20)
)

type ARVISMemoryReadRequest struct {
	Target  string
	Network string
	Since   time.Time
	Until   time.Time
	Limit   uint64
}

type ARVISVerdictMemory struct {
	VerdictID       string          `json:"verdict_id"`
	EventID         string          `json:"event_id,omitempty"`
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
	Signed          bool            `json:"signed"`
	Signature       string          `json:"signature,omitempty"`
	Source          string          `json:"source"`
	EventType       string          `json:"event_type,omitempty"`
	Provider        string          `json:"provider,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	SourceUpdatedAt time.Time       `json:"source_updated_at"`
	SourceVersion   uint64          `json:"source_version"`
	SchemaVersion   uint16          `json:"schema_version"`
}

type ARVISStreamMemory struct {
	EventID         string    `json:"event_id"`
	EventKey        string    `json:"event_key"`
	Provider        string    `json:"provider"`
	StreamMode      string    `json:"stream_mode"`
	Network         string    `json:"network"`
	ModuleID        string    `json:"module_id"`
	EventType       string    `json:"event_type"`
	Target          string    `json:"target"`
	TargetType      string    `json:"target_type"`
	Signature       string    `json:"signature,omitempty"`
	Slot            uint64    `json:"slot"`
	ProgramID       string    `json:"program_id,omitempty"`
	EvidenceQuality string    `json:"evidence_quality"`
	DecodedSHA256   string    `json:"decoded_sha256"`
	RawEventSHA256  string    `json:"raw_event_sha256"`
	CreatedAt       time.Time `json:"created_at"`
	IngestVersion   uint64    `json:"ingest_version"`
	SchemaVersion   uint16    `json:"schema_version"`
}

type ARVISEvidenceLink struct {
	VerdictID   string   `json:"verdict_id"`
	EventID     string   `json:"event_id,omitempty"`
	Status      string   `json:"status"`
	EvidenceRef []string `json:"evidence_refs"`
}

type ARVISMemorySnapshot struct {
	Target                  string               `json:"target"`
	Network                 string               `json:"network"`
	Since                   time.Time            `json:"since"`
	Until                   time.Time            `json:"until"`
	CorrelationStatus       string               `json:"correlation_status"`
	VerdictContentIntegrity string               `json:"verdict_content_integrity"`
	StreamPayloadPolicy     string               `json:"stream_payload_policy"`
	Verdicts                []ARVISVerdictMemory `json:"verdicts"`
	Events                  []ARVISStreamMemory  `json:"events"`
	Links                   []ARVISEvidenceLink  `json:"links"`
	LinkedVerdicts          int                  `json:"linked_verdicts"`
	StandaloneVerdicts      int                  `json:"standalone_verdicts"`
	MissingLinkedEvents     int                  `json:"missing_linked_events"`
	UnreferencedEvents      int                  `json:"unreferenced_events"`
	FingerprintSHA256       string               `json:"fingerprint_sha256"`
	Limitations             []string             `json:"limitations"`
}

type arvisVerdictMemoryRow struct {
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

type arvisStreamMemoryRow struct {
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
	CreatedAtMillis int64  `json:"created_at_ms"`
	IngestVersion   uint64 `json:"ingest_version"`
	SchemaVersion   uint16 `json:"schema_version"`
}

func (c *Client) ReadARVISMemory(ctx context.Context, request ARVISMemoryReadRequest) (ARVISMemorySnapshot, error) {
	normalized, err := normalizeARVISMemoryReadRequest(request)
	if err != nil {
		return ARVISMemorySnapshot{}, err
	}
	verdicts, err := c.readARVISVerdictMemory(ctx, normalized)
	if err != nil {
		return ARVISMemorySnapshot{}, err
	}
	events, err := c.readARVISStreamMemory(ctx, normalized)
	if err != nil {
		return ARVISMemorySnapshot{}, err
	}
	return correlateARVISMemory(normalized, verdicts, events), nil
}

func normalizeARVISMemoryReadRequest(request ARVISMemoryReadRequest) (ARVISMemoryReadRequest, error) {
	request.Target = strings.TrimSpace(request.Target)
	request.Network = strings.TrimSpace(request.Network)
	request.Since = request.Since.UTC()
	request.Until = request.Until.UTC()
	if request.Target == "" {
		return ARVISMemoryReadRequest{}, fmt.Errorf("ARVIS memory target is required")
	}
	if request.Network == "" {
		return ARVISMemoryReadRequest{}, fmt.Errorf("ARVIS memory network is required")
	}
	if request.Since.IsZero() || request.Until.IsZero() || !request.Since.Before(request.Until) {
		return ARVISMemoryReadRequest{}, fmt.Errorf("ARVIS memory read window is invalid")
	}
	if request.Until.Sub(request.Since) > MaxARVISMemoryReadWindow {
		return ARVISMemoryReadRequest{}, fmt.Errorf("ARVIS memory read window exceeds %s", MaxARVISMemoryReadWindow)
	}
	if request.Limit == 0 || request.Limit > MaxARVISMemoryReadRows {
		return ARVISMemoryReadRequest{}, fmt.Errorf("ARVIS memory row limit must be between 1 and %d", MaxARVISMemoryReadRows)
	}
	return request, nil
}

func (c *Client) readARVISVerdictMemory(ctx context.Context, request ARVISMemoryReadRequest) ([]ARVISVerdictMemory, error) {
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
WHERE target = {target:String}
  AND network = {network:String}
  AND source_updated_at >= {since:DateTime64(6)}
  AND source_updated_at < {until:DateTime64(6)}
ORDER BY source_updated_at ASC, module_id ASC, toString(verdict_id) ASC
LIMIT {limit:UInt64}
FORMAT JSONEachRow`)
	setARVISMemoryQueryParams(params, request, true)
	queryURL.RawQuery = params.Encode()
	rows, err := queryARVISMemoryRows[arvisVerdictMemoryRow](ctx, c, queryURL.String(), request.Limit)
	if err != nil {
		return nil, fmt.Errorf("read ClickHouse ARVIS verdict memory: %w", err)
	}
	out := make([]ARVISVerdictMemory, 0, len(rows))
	for _, row := range rows {
		item, err := validateARVISVerdictMemoryRow(row, request)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (c *Client) readARVISStreamMemory(ctx context.Context, request ARVISMemoryReadRequest) ([]ARVISStreamMemory, error) {
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
    toUnixTimestamp64Milli(created_at) AS created_at_ms,
    ingest_version,
    schema_version
FROM security_radar_stream_events FINAL
WHERE network = {network:String}
  AND target = {target:String}
  AND event_date >= toDate({since:DateTime64(3)})
  AND event_date <= toDate({until:DateTime64(3)})
  AND created_at >= {since:DateTime64(3)}
  AND created_at < {until:DateTime64(3)}
ORDER BY created_at ASC, module_id ASC, toString(event_id) ASC
LIMIT {limit:UInt64}
FORMAT JSONEachRow`)
	setARVISMemoryQueryParams(params, request, false)
	queryURL.RawQuery = params.Encode()
	rows, err := queryARVISMemoryRows[arvisStreamMemoryRow](ctx, c, queryURL.String(), request.Limit)
	if err != nil {
		return nil, fmt.Errorf("read ClickHouse ARVIS stream memory: %w", err)
	}
	out := make([]ARVISStreamMemory, 0, len(rows))
	for _, row := range rows {
		item, err := validateARVISStreamMemoryRow(row, request)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func setARVISMemoryQueryParams(params mapLikeValues, request ARVISMemoryReadRequest, micros bool) {
	params.Set("param_target", request.Target)
	params.Set("param_network", request.Network)
	if micros {
		params.Set("param_since", parityTimeMicrosParam(request.Since))
		params.Set("param_until", parityTimeMicrosParam(request.Until))
	} else {
		params.Set("param_since", parityTimeParam(request.Since))
		params.Set("param_until", parityTimeParam(request.Until))
	}
	params.Set("param_limit", strconv.FormatUint(request.Limit+1, 10))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(arvisMemoryScanRowCap, 10))
	params.Set("max_result_rows", strconv.FormatUint(request.Limit+1, 10))
	params.Set("result_overflow_mode", "throw")
}

type mapLikeValues interface {
	Set(string, string)
}

func queryARVISMemoryRows[T any](ctx context.Context, c *Client, rawURL string, limit uint64) ([]T, error) {
	if c == nil {
		return nil, fmt.Errorf("ClickHouse client is unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ClickHouse query failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	limited := &io.LimitedReader{R: resp.Body, N: arvisMemoryBodyLimit + 1}
	decoder := json.NewDecoder(limited)
	rows := make([]T, 0)
	for {
		var row T
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode ClickHouse ARVIS memory row: %w", err)
		}
		if uint64(len(rows)) >= limit {
			return nil, fmt.Errorf("ClickHouse ARVIS memory read exceeded row limit %d", limit)
		}
		rows = append(rows, row)
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("ClickHouse ARVIS memory response exceeded %d bytes", arvisMemoryBodyLimit)
	}
	return rows, nil
}

func validateARVISVerdictMemoryRow(row arvisVerdictMemoryRow, request ARVISMemoryReadRequest) (ARVISVerdictMemory, error) {
	row.VerdictID = strings.ToLower(strings.TrimSpace(row.VerdictID))
	row.EventID = strings.ToLower(strings.TrimSpace(row.EventID))
	if !uuidRE.MatchString(row.VerdictID) || !uuidRE.MatchString(row.EventID) {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict memory contains invalid UUID identity")
	}
	if row.Target != request.Target || row.Network != request.Network {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict memory escaped exact subject boundary")
	}
	if row.SchemaVersion != VerdictSnapshotSchemaVersion {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict %s has unsupported schema version %d", row.VerdictID, row.SchemaVersion)
	}
	if row.Signed > 1 {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict %s has invalid signed state", row.VerdictID)
	}
	row.EvidenceSHA256 = strings.TrimSpace(row.EvidenceSHA256)
	row.SignalsSHA256 = strings.TrimSpace(row.SignalsSHA256)
	if !hex64RE.MatchString(row.EvidenceSHA256) || !hex64RE.MatchString(row.SignalsSHA256) {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict %s has invalid content digest", row.VerdictID)
	}
	if got := evidenceListSHA256(row.Evidence); got != row.EvidenceSHA256 {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict %s evidence hash mismatch", row.VerdictID)
	}
	canonicalSignals, err := canonicalJSON(row.Signals)
	if err != nil {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict %s signals are invalid: %w", row.VerdictID, err)
	}
	if got := jsonPayloadSHA256(canonicalSignals); got != row.SignalsSHA256 {
		return ARVISVerdictMemory{}, fmt.Errorf("ClickHouse ARVIS verdict %s signals hash mismatch", row.VerdictID)
	}
	return ARVISVerdictMemory{
		VerdictID:       row.VerdictID,
		EventID:         nonZeroUUID(row.EventID),
		ModuleID:        strings.TrimSpace(row.ModuleID),
		Target:          row.Target,
		TargetType:      strings.TrimSpace(row.TargetType),
		Network:         row.Network,
		Grade:           strings.TrimSpace(row.Grade),
		RiskIndex:       row.RiskIndex,
		RiskLevel:       strings.TrimSpace(row.RiskLevel),
		Verdict:         strings.TrimSpace(row.Verdict),
		Recommendation:  row.Recommendation,
		Evidence:        append([]string(nil), row.Evidence...),
		EvidenceSHA256:  row.EvidenceSHA256,
		Signals:         canonicalSignals,
		SignalsSHA256:   row.SignalsSHA256,
		RuleVersion:     strings.TrimSpace(row.RuleVersion),
		Signed:          row.Signed == 1,
		Signature:       strings.TrimSpace(row.Signature),
		Source:          strings.TrimSpace(row.Source),
		EventType:       strings.TrimSpace(row.EventType),
		Provider:        strings.TrimSpace(row.Provider),
		CreatedAt:       time.UnixMicro(row.CreatedAtMicros).UTC(),
		SourceUpdatedAt: time.UnixMicro(row.SourceUpdatedAtMicros).UTC(),
		SourceVersion:   row.SourceVersion,
		SchemaVersion:   row.SchemaVersion,
	}, nil
}

func validateARVISStreamMemoryRow(row arvisStreamMemoryRow, request ARVISMemoryReadRequest) (ARVISStreamMemory, error) {
	row.EventID = strings.ToLower(strings.TrimSpace(row.EventID))
	if !uuidRE.MatchString(row.EventID) {
		return ARVISStreamMemory{}, fmt.Errorf("ClickHouse ARVIS stream memory contains invalid event UUID")
	}
	if row.Target != request.Target || row.Network != request.Network {
		return ARVISStreamMemory{}, fmt.Errorf("ClickHouse ARVIS stream memory escaped exact subject boundary")
	}
	if row.SchemaVersion != StreamSchemaVersion {
		return ARVISStreamMemory{}, fmt.Errorf("ClickHouse ARVIS stream event %s has unsupported schema version %d", row.EventID, row.SchemaVersion)
	}
	row.EventKey = strings.TrimSpace(row.EventKey)
	row.DecodedSHA256 = strings.TrimSpace(row.DecodedSHA256)
	row.RawEventSHA256 = strings.TrimSpace(row.RawEventSHA256)
	if !hex64RE.MatchString(row.EventKey) || !hex64RE.MatchString(row.DecodedSHA256) || !hex64RE.MatchString(row.RawEventSHA256) {
		return ARVISStreamMemory{}, fmt.Errorf("ClickHouse ARVIS stream event %s has invalid digest metadata", row.EventID)
	}
	return ARVISStreamMemory{
		EventID:         row.EventID,
		EventKey:        row.EventKey,
		Provider:        strings.TrimSpace(row.Provider),
		StreamMode:      strings.TrimSpace(row.StreamMode),
		Network:         row.Network,
		ModuleID:        strings.TrimSpace(row.ModuleID),
		EventType:       strings.TrimSpace(row.EventType),
		Target:          row.Target,
		TargetType:      strings.TrimSpace(row.TargetType),
		Signature:       strings.TrimSpace(row.Signature),
		Slot:            row.Slot,
		ProgramID:       strings.TrimSpace(row.ProgramID),
		EvidenceQuality: strings.TrimSpace(row.EvidenceQuality),
		DecodedSHA256:   row.DecodedSHA256,
		RawEventSHA256:  row.RawEventSHA256,
		CreatedAt:       time.UnixMilli(row.CreatedAtMillis).UTC(),
		IngestVersion:   row.IngestVersion,
		SchemaVersion:   row.SchemaVersion,
	}, nil
}

func correlateARVISMemory(request ARVISMemoryReadRequest, verdicts []ARVISVerdictMemory, events []ARVISStreamMemory) ARVISMemorySnapshot {
	eventsByID := make(map[string]ARVISStreamMemory, len(events))
	for _, event := range events {
		eventsByID[event.EventID] = event
	}
	referencedEvents := map[string]bool{}
	links := make([]ARVISEvidenceLink, 0, len(verdicts))
	linked := 0
	standalone := 0
	missing := 0
	for _, verdict := range verdicts {
		link := ARVISEvidenceLink{VerdictID: verdict.VerdictID, EventID: verdict.EventID, EvidenceRef: []string{"clickhouse:verdict:" + verdict.VerdictID}}
		switch {
		case verdict.EventID == "":
			link.Status = "no_event_reference"
			standalone++
		case eventsByID[verdict.EventID].EventID != "":
			link.Status = "linked"
			link.EvidenceRef = append(link.EvidenceRef, "clickhouse:event:"+verdict.EventID)
			referencedEvents[verdict.EventID] = true
			linked++
		default:
			link.Status = "event_not_observed_in_window"
			missing++
		}
		links = append(links, link)
	}
	unreferenced := 0
	for _, event := range events {
		if !referencedEvents[event.EventID] {
			unreferenced++
		}
	}
	status := "correlated"
	if len(verdicts) == 0 && len(events) == 0 {
		status = "empty"
	} else if missing > 0 {
		status = "partial"
	}
	return ARVISMemorySnapshot{
		Target:                  request.Target,
		Network:                 request.Network,
		Since:                   request.Since,
		Until:                   request.Until,
		CorrelationStatus:       status,
		VerdictContentIntegrity: "sha256_verified",
		StreamPayloadPolicy:     "digests_only_not_dereferenced",
		Verdicts:                verdicts,
		Events:                  events,
		Links:                   links,
		LinkedVerdicts:          linked,
		StandaloneVerdicts:      standalone,
		MissingLinkedEvents:     missing,
		UnreferencedEvents:      unreferenced,
		FingerprintSHA256:       arvisMemoryFingerprint(verdicts, events),
		Limitations: []string{
			"This is a bounded read-only ClickHouse memory view; it does not change ARVIS decisions or runtime state.",
			"Verdict evidence and signals are re-hashed on read; stream payload bytes are not dereferenced in this view and are represented by stored SHA-256 digests.",
			"Event correlation uses only exact event UUID references inside the requested exact target/network/time window; no relationship is inferred from co-occurrence.",
		},
	}
}

func arvisMemoryFingerprint(verdicts []ARVISVerdictMemory, events []ARVISStreamMemory) string {
	h := sha256.New()
	for _, verdict := range verdicts {
		writeLengthPrefixedFields(h, []string{
			"verdict", verdict.VerdictID, verdict.EventID, verdict.ModuleID, verdict.Target, verdict.Network,
			verdict.EvidenceSHA256, verdict.SignalsSHA256, strconv.FormatUint(verdict.SourceVersion, 10), strconv.FormatUint(uint64(verdict.SchemaVersion), 10),
		})
	}
	for _, event := range events {
		writeLengthPrefixedFields(h, []string{
			"event", event.EventID, event.EventKey, event.ModuleID, event.Target, event.Network,
			event.DecodedSHA256, event.RawEventSHA256, strconv.FormatUint(event.IngestVersion, 10), strconv.FormatUint(uint64(event.SchemaVersion), 10),
		})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func nonZeroUUID(value string) string {
	if value == zeroUUID {
		return ""
	}
	return value
}
