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

	"koschei/api/internal/services"
)

const (
	MaxGlobalRadarGraphReadRows   = uint64(5000)
	MaxGlobalRadarGraphReadWindow = 31 * 24 * time.Hour
	globalRadarGraphScanRowCap     = uint64(10000000)
	globalRadarGraphBodyLimit      = int64(64 << 20)
)

type globalRadarGraphRecordRow struct {
	RecordType       string   `json:"record_type"`
	RecordID         string   `json:"record_id"`
	SchemaVersion    string   `json:"schema_version"`
	SnapshotSHA256   string   `json:"snapshot_sha256"`
	Network          string   `json:"network"`
	SecondaryNetwork string   `json:"secondary_network"`
	SubjectID        string   `json:"subject_id"`
	TargetSubjectID  string   `json:"target_subject_id"`
	RecordKind       string   `json:"record_kind"`
	EvidenceStatus   string   `json:"evidence_status"`
	EvidenceRefs     []string `json:"evidence_refs"`
	ObservedAt       any      `json:"observed_at"`
	RecordedAt       time.Time `json:"recorded_at"`
	PayloadJSON      string   `json:"payload_json"`
	PayloadSHA256    string   `json:"payload_sha256"`
	IngestVersion    uint64   `json:"ingest_version"`
}

type globalRadarGraphReadRow struct {
	RecordType       string   `json:"record_type"`
	RecordID         string   `json:"record_id"`
	SchemaVersion    string   `json:"schema_version"`
	SnapshotSHA256   string   `json:"snapshot_sha256"`
	Network          string   `json:"network"`
	SecondaryNetwork string   `json:"secondary_network"`
	SubjectID        string   `json:"subject_id"`
	TargetSubjectID  string   `json:"target_subject_id"`
	RecordKind       string   `json:"record_kind"`
	EvidenceStatus   string   `json:"evidence_status"`
	EvidenceRefs     []string `json:"evidence_refs"`
	ObservedAtNanos  int64    `json:"observed_at_ns"`
	RecordedAtNanos  int64    `json:"recorded_at_ns"`
	PayloadJSON      string   `json:"payload_json"`
	PayloadSHA256    string   `json:"payload_sha256"`
	IngestVersion    uint64   `json:"ingest_version"`
}

type GlobalRadarGraphReadRequest struct {
	Network    string
	SubjectID  string
	RecordType string
	Since      time.Time
	Until      time.Time
	Limit      uint64
}

type GlobalRadarStoredRecord struct {
	RecordType       string          `json:"record_type"`
	RecordID         string          `json:"record_id"`
	SchemaVersion    string          `json:"schema_version"`
	SnapshotSHA256   string          `json:"snapshot_sha256"`
	Network          string          `json:"network"`
	SecondaryNetwork string          `json:"secondary_network,omitempty"`
	SubjectID        string          `json:"subject_id"`
	TargetSubjectID  string          `json:"target_subject_id,omitempty"`
	RecordKind       string          `json:"record_kind"`
	EvidenceStatus   string          `json:"evidence_status"`
	EvidenceRefs     []string        `json:"evidence_refs,omitempty"`
	ObservedAt       time.Time       `json:"observed_at,omitempty"`
	RecordedAt       time.Time       `json:"recorded_at"`
	Payload          json.RawMessage `json:"payload"`
	PayloadSHA256    string          `json:"payload_sha256"`
	IngestVersion    uint64          `json:"ingest_version"`
}

func (c *Client) InsertGlobalRadarSnapshot(ctx context.Context, snapshot services.GlobalRadarSnapshot) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	ingestVersion := uint64(time.Now().UTC().UnixNano())
	rows, err := globalRadarRowsFromSnapshot(snapshot, ingestVersion)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("encode ClickHouse Global Radar record: %w", err)
		}
	}

	insertURL := *c.endpoint
	query := insertURL.Query()
	query.Set("query", "INSERT INTO global_radar_graph_records FORMAT JSONEachRow")
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "1")
	query.Set("date_time_input_format", "best_effort")
	insertURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, insertURL.String(), &body)
	if err != nil {
		return fmt.Errorf("build ClickHouse Global Radar insert request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse Global Radar insert request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("ClickHouse Global Radar insert failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
}

func globalRadarRowsFromSnapshot(snapshot services.GlobalRadarSnapshot, ingestVersion uint64) ([]globalRadarGraphRecordRow, error) {
	if snapshot.SchemaVersion != services.GlobalRadarSnapshotSchemaVersion || snapshot.GeneratedAt.IsZero() {
		return nil, fmt.Errorf("valid Global Radar snapshot is required")
	}
	rebuilt, err := services.BuildGlobalRadarSnapshot(
		snapshot.Observations,
		snapshot.Relations,
		snapshot.BridgeLinks,
		snapshot.VerdictRefs,
		snapshot.GeneratedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("revalidate Global Radar snapshot: %w", err)
	}
	if !reflect.DeepEqual(snapshot.Coverage, rebuilt.Coverage) {
		return nil, fmt.Errorf("Global Radar snapshot coverage does not match canonical rebuild")
	}
	snapshotBytes, err := json.Marshal(rebuilt)
	if err != nil {
		return nil, fmt.Errorf("encode Global Radar snapshot: %w", err)
	}
	snapshotSHA256 := jsonPayloadSHA256(snapshotBytes)
	recordedAt := rebuilt.GeneratedAt.UTC()
	rows := make([]globalRadarGraphRecordRow, 0, len(rebuilt.Observations)+len(rebuilt.Relations)+len(rebuilt.BridgeLinks)+len(rebuilt.VerdictRefs))

	for _, observation := range rebuilt.Observations {
		observedAt := observation.ObservedAt.UTC()
		row, err := newGlobalRadarGraphRecordRow(
			"observation",
			observation.ObservationID,
			observation.SchemaVersion,
			snapshotSHA256,
			observation.Subject.Network,
			"",
			observation.Subject.ID,
			"",
			observation.ObservationKind,
			observation.Evidence.Status,
			[]string{observation.Evidence.ID},
			&observedAt,
			recordedAt,
			observation,
			ingestVersion,
		)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, relation := range rebuilt.Relations {
		row, err := newGlobalRadarGraphRecordRow(
			"relation",
			relation.EdgeID,
			relation.SchemaVersion,
			snapshotSHA256,
			relation.Source.Network,
			relation.Target.Network,
			relation.Source.ID,
			relation.Target.ID,
			relation.Relation,
			relation.Status,
			relation.EvidenceRefs,
			nil,
			recordedAt,
			relation,
			ingestVersion,
		)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, link := range rebuilt.BridgeLinks {
		observedAt := link.LinkEvidence.ObservedAt.UTC()
		row, err := newGlobalRadarGraphRecordRow(
			"bridge_link",
			link.LinkID,
			link.SchemaVersion,
			snapshotSHA256,
			link.SourceObservation.Subject.Network,
			link.DestinationObservation.Subject.Network,
			link.SourceObservation.Subject.ID,
			link.DestinationObservation.Subject.ID,
			link.BridgeProtocol,
			link.LinkEvidence.Status,
			[]string{link.LinkEvidence.ID, link.SourceObservation.Evidence.ID, link.DestinationObservation.Evidence.ID},
			&observedAt,
			recordedAt,
			link,
			ingestVersion,
		)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, reference := range rebuilt.VerdictRefs {
		observedAt := reference.GeneratedAt.UTC()
		row, err := newGlobalRadarGraphRecordRow(
			"verdict_reference",
			reference.ReferenceID,
			reference.SchemaVersion,
			snapshotSHA256,
			reference.Network,
			"",
			reference.TargetSubjectID,
			"",
			"arvis_verdict_reference",
			reference.DecisionEvidenceStatus,
			reference.EvidenceRefs,
			&observedAt,
			recordedAt,
			reference,
			ingestVersion,
		)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func newGlobalRadarGraphRecordRow(
	recordType, recordID, schemaVersion, snapshotSHA256, network, secondaryNetwork, subjectID, targetSubjectID, recordKind, evidenceStatus string,
	evidenceRefs []string,
	observedAt *time.Time,
	recordedAt time.Time,
	payload any,
	ingestVersion uint64,
) (globalRadarGraphRecordRow, error) {
	if !globalRadarRecordTypeAllowed(recordType) ||
		strings.TrimSpace(recordID) == "" ||
		strings.TrimSpace(schemaVersion) == "" ||
		strings.TrimSpace(network) == "" ||
		strings.TrimSpace(subjectID) == "" ||
		strings.TrimSpace(recordKind) == "" ||
		!hex64RE.MatchString(strings.TrimSpace(snapshotSHA256)) ||
		recordedAt.IsZero() {
		return globalRadarGraphRecordRow{}, fmt.Errorf("invalid Global Radar storage record identity")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return globalRadarGraphRecordRow{}, fmt.Errorf("encode Global Radar storage payload: %w", err)
	}
	if !json.Valid(payloadBytes) {
		return globalRadarGraphRecordRow{}, fmt.Errorf("Global Radar storage payload is invalid JSON")
	}
	var observed any
	if observedAt != nil && !observedAt.IsZero() {
		observed = observedAt.UTC()
	}
	return globalRadarGraphRecordRow{
		RecordType:       recordType,
		RecordID:         strings.TrimSpace(recordID),
		SchemaVersion:    strings.TrimSpace(schemaVersion),
		SnapshotSHA256:   strings.TrimSpace(snapshotSHA256),
		Network:          strings.TrimSpace(network),
		SecondaryNetwork: strings.TrimSpace(secondaryNetwork),
		SubjectID:        strings.TrimSpace(subjectID),
		TargetSubjectID:  strings.TrimSpace(targetSubjectID),
		RecordKind:       strings.TrimSpace(recordKind),
		EvidenceStatus:   strings.TrimSpace(evidenceStatus),
		EvidenceRefs:     nonEmptyStrings(evidenceRefs),
		ObservedAt:       observed,
		RecordedAt:       recordedAt.UTC(),
		PayloadJSON:      string(payloadBytes),
		PayloadSHA256:    jsonPayloadSHA256(payloadBytes),
		IngestVersion:    ingestVersion,
	}, nil
}

func (c *Client) ReadGlobalRadarGraph(ctx context.Context, request GlobalRadarGraphReadRequest) ([]GlobalRadarStoredRecord, error) {
	if c == nil {
		return nil, fmt.Errorf("ClickHouse client is unavailable")
	}
	request, err := normalizeGlobalRadarGraphReadRequest(request)
	if err != nil {
		return nil, err
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	queryText := `SELECT
    record_type,
    record_id,
    schema_version,
    toString(snapshot_sha256) AS snapshot_sha256,
    network,
    secondary_network,
    subject_id,
    target_subject_id,
    record_kind,
    evidence_status,
    evidence_refs,
    if(isNull(observed_at), toInt64(0), toUnixTimestamp64Nano(assumeNotNull(observed_at))) AS observed_at_ns,
    toUnixTimestamp64Nano(recorded_at) AS recorded_at_ns,
    payload_json,
    toString(payload_sha256) AS payload_sha256,
    ingest_version
FROM global_radar_graph_records FINAL
WHERE record_date >= toDate({since:DateTime64(9)})
  AND record_date <= toDate({until:DateTime64(9)})
  AND recorded_at >= {since:DateTime64(9)}
  AND recorded_at < {until:DateTime64(9)}
  AND (network = {network:String} OR secondary_network = {network:String})`
	if request.SubjectID != "" {
		queryText += "\n  AND (subject_id = {subject:String} OR target_subject_id = {subject:String})"
	}
	if request.RecordType != "" {
		queryText += "\n  AND record_type = {record_type:String}"
	}
	queryText += "\nORDER BY recorded_at ASC, record_type ASC, record_id ASC, payload_sha256 ASC\nLIMIT {limit:UInt64}\nFORMAT JSONEachRow"
	params.Set("query", queryText)
	params.Set("param_since", globalRadarTimeParam(request.Since))
	params.Set("param_until", globalRadarTimeParam(request.Until))
	params.Set("param_network", request.Network)
	if request.SubjectID != "" {
		params.Set("param_subject", request.SubjectID)
	}
	if request.RecordType != "" {
		params.Set("param_record_type", request.RecordType)
	}
	params.Set("param_limit", strconv.FormatUint(request.Limit+1, 10))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(globalRadarGraphScanRowCap, 10))
	params.Set("max_result_rows", strconv.FormatUint(request.Limit+1, 10))
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build ClickHouse Global Radar read request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ClickHouse Global Radar read request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ClickHouse Global Radar read failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	limited := &io.LimitedReader{R: resp.Body, N: globalRadarGraphBodyLimit + 1}
	decoder := json.NewDecoder(limited)
	out := make([]GlobalRadarStoredRecord, 0)
	for {
		var row globalRadarGraphReadRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode ClickHouse Global Radar row: %w", err)
		}
		if uint64(len(out)) >= request.Limit {
			return nil, fmt.Errorf("ClickHouse Global Radar read exceeded row limit %d", request.Limit)
		}
		item, err := validateGlobalRadarGraphReadRow(row, request)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("ClickHouse Global Radar response exceeded %d bytes", globalRadarGraphBodyLimit)
	}
	return out, nil
}

func normalizeGlobalRadarGraphReadRequest(request GlobalRadarGraphReadRequest) (GlobalRadarGraphReadRequest, error) {
	request.Network = strings.TrimSpace(request.Network)
	request.SubjectID = strings.TrimSpace(request.SubjectID)
	request.RecordType = strings.ToLower(strings.TrimSpace(request.RecordType))
	request.Since = request.Since.UTC()
	request.Until = request.Until.UTC()
	if request.Network == "" {
		return GlobalRadarGraphReadRequest{}, fmt.Errorf("Global Radar network is required")
	}
	if request.Since.IsZero() || request.Until.IsZero() || !request.Since.Before(request.Until) {
		return GlobalRadarGraphReadRequest{}, fmt.Errorf("Global Radar read window is invalid")
	}
	if request.Until.Sub(request.Since) > MaxGlobalRadarGraphReadWindow {
		return GlobalRadarGraphReadRequest{}, fmt.Errorf("Global Radar read window exceeds %s", MaxGlobalRadarGraphReadWindow)
	}
	if request.Limit == 0 || request.Limit > MaxGlobalRadarGraphReadRows {
		return GlobalRadarGraphReadRequest{}, fmt.Errorf("Global Radar row limit must be between 1 and %d", MaxGlobalRadarGraphReadRows)
	}
	if request.RecordType != "" && !globalRadarRecordTypeAllowed(request.RecordType) {
		return GlobalRadarGraphReadRequest{}, fmt.Errorf("unsupported Global Radar record type")
	}
	return request, nil
}

func validateGlobalRadarGraphReadRow(row globalRadarGraphReadRow, request GlobalRadarGraphReadRequest) (GlobalRadarStoredRecord, error) {
	if !globalRadarRecordTypeAllowed(row.RecordType) ||
		strings.TrimSpace(row.RecordID) == "" ||
		strings.TrimSpace(row.SchemaVersion) == "" ||
		strings.TrimSpace(row.SubjectID) == "" ||
		!hex64RE.MatchString(strings.TrimSpace(row.SnapshotSHA256)) ||
		!hex64RE.MatchString(strings.TrimSpace(row.PayloadSHA256)) {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row has invalid identity metadata")
	}
	if row.Network != request.Network && row.SecondaryNetwork != request.Network {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row escaped requested network boundary")
	}
	if request.SubjectID != "" && row.SubjectID != request.SubjectID && row.TargetSubjectID != request.SubjectID {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row escaped requested subject boundary")
	}
	if request.RecordType != "" && row.RecordType != request.RecordType {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row escaped requested record-type boundary")
	}
	recordedAt := time.Unix(0, row.RecordedAtNanos).UTC()
	if recordedAt.Before(request.Since) || !recordedAt.Before(request.Until) {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row escaped requested time boundary")
	}
	payload := json.RawMessage(row.PayloadJSON)
	if !json.Valid(payload) {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row contains invalid payload JSON")
	}
	if got := jsonPayloadSHA256(payload); got != row.PayloadSHA256 {
		return GlobalRadarStoredRecord{}, fmt.Errorf("ClickHouse Global Radar row payload hash mismatch")
	}
	var observedAt time.Time
	if row.ObservedAtNanos != 0 {
		observedAt = time.Unix(0, row.ObservedAtNanos).UTC()
	}
	return GlobalRadarStoredRecord{
		RecordType:       row.RecordType,
		RecordID:         strings.TrimSpace(row.RecordID),
		SchemaVersion:    strings.TrimSpace(row.SchemaVersion),
		SnapshotSHA256:   strings.TrimSpace(row.SnapshotSHA256),
		Network:          row.Network,
		SecondaryNetwork: row.SecondaryNetwork,
		SubjectID:        row.SubjectID,
		TargetSubjectID:  row.TargetSubjectID,
		RecordKind:       row.RecordKind,
		EvidenceStatus:   row.EvidenceStatus,
		EvidenceRefs:     nonEmptyStrings(row.EvidenceRefs),
		ObservedAt:       observedAt,
		RecordedAt:       recordedAt,
		Payload:          append(json.RawMessage(nil), payload...),
		PayloadSHA256:    row.PayloadSHA256,
		IngestVersion:    row.IngestVersion,
	}, nil
}

func globalRadarRecordTypeAllowed(value string) bool {
	switch value {
	case "observation", "relation", "bridge_link", "verdict_reference":
		return true
	default:
		return false
	}
}

func globalRadarTimeParam(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05.999999999")
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
