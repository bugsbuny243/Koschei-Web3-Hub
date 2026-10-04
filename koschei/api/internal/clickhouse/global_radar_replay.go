package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const globalRadarReplayMaxSnapshots = 100
const globalRadarReplayMaxRowsPerSnapshot = uint64(512)

type globalRadarReplayHeadRow struct {
	SnapshotSHA256  string `json:"snapshot_sha256"`
	RecordedAtNanos int64  `json:"recorded_at_ns"`
}

// ReadGlobalCampaignReplay reconstructs canonical Global Radar snapshots from
// the append-first ClickHouse graph ledger. It is intentionally read-only and
// bounded; PostgreSQL owns queue acknowledgement and the durable replay cursor.
func (c *Client) ReadGlobalCampaignReplay(ctx context.Context, cursor services.GlobalCampaignReplayCursor, limit int) ([]services.GlobalCampaignReplayItem, error) {
	if c == nil {
		return nil, fmt.Errorf("ClickHouse client is unavailable")
	}
	if limit <= 0 || limit > globalRadarReplayMaxSnapshots {
		limit = 25
	}
	cursor.RecordedAt = cursor.RecordedAt.UTC()
	cursor.SnapshotSHA256 = strings.TrimSpace(cursor.SnapshotSHA256)
	if cursor.RecordedAt.IsZero() {
		return nil, fmt.Errorf("Global Radar replay cursor time is required")
	}

	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT
    toString(snapshot_sha256) AS snapshot_sha256,
    toUnixTimestamp64Nano(min(recorded_at)) AS recorded_at_ns
FROM global_radar_graph_records FINAL
WHERE recorded_at > {after:DateTime64(9)}
   OR (recorded_at = {after:DateTime64(9)} AND toString(snapshot_sha256) > {after_hash:String})
GROUP BY snapshot_sha256
ORDER BY recorded_at_ns ASC, snapshot_sha256 ASC
LIMIT {limit:UInt64}
FORMAT JSONEachRow`)
	params.Set("param_after", globalRadarTimeParam(cursor.RecordedAt))
	params.Set("param_after_hash", cursor.SnapshotSHA256)
	params.Set("param_limit", strconv.Itoa(limit+1))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(globalRadarGraphScanRowCap, 10))
	params.Set("max_result_rows", strconv.Itoa(limit+1))
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build ClickHouse Global Radar replay-head request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ClickHouse Global Radar replay-head request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ClickHouse Global Radar replay-head failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	limited := &io.LimitedReader{R: resp.Body, N: globalRadarGraphBodyLimit + 1}
	decoder := json.NewDecoder(limited)
	heads := make([]globalRadarReplayHeadRow, 0, limit)
	for {
		var row globalRadarReplayHeadRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode ClickHouse Global Radar replay head: %w", err)
		}
		if len(heads) >= limit {
			return nil, fmt.Errorf("ClickHouse Global Radar replay exceeded snapshot limit %d", limit)
		}
		if !hex64RE.MatchString(strings.TrimSpace(row.SnapshotSHA256)) || row.RecordedAtNanos <= 0 {
			return nil, fmt.Errorf("ClickHouse Global Radar replay head has invalid identity")
		}
		heads = append(heads, row)
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("ClickHouse Global Radar replay-head response exceeded %d bytes", globalRadarGraphBodyLimit)
	}

	out := make([]services.GlobalCampaignReplayItem, 0, len(heads))
	for _, head := range heads {
		recordedAt := time.Unix(0, head.RecordedAtNanos).UTC()
		snapshot, err := c.readGlobalRadarSnapshotForReplay(ctx, strings.TrimSpace(head.SnapshotSHA256), recordedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, services.GlobalCampaignReplayItem{
			Cursor: services.GlobalCampaignReplayCursor{RecordedAt: recordedAt, SnapshotSHA256: strings.TrimSpace(head.SnapshotSHA256)},
			Snapshot: snapshot,
		})
	}
	return out, nil
}

func (c *Client) readGlobalRadarSnapshotForReplay(ctx context.Context, snapshotSHA256 string, recordedAt time.Time) (services.GlobalRadarSnapshot, error) {
	queryURL := *c.endpoint
	params := queryURL.Query()
	params.Set("query", `SELECT
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
WHERE toString(snapshot_sha256) = {snapshot:String}
ORDER BY record_type ASC, record_id ASC, payload_sha256 ASC
LIMIT {limit:UInt64}
FORMAT JSONEachRow`)
	params.Set("param_snapshot", snapshotSHA256)
	params.Set("param_limit", strconv.FormatUint(globalRadarReplayMaxRowsPerSnapshot+1, 10))
	params.Set("max_execution_time", "30")
	params.Set("max_rows_to_read", strconv.FormatUint(globalRadarGraphScanRowCap, 10))
	params.Set("max_result_rows", strconv.FormatUint(globalRadarReplayMaxRowsPerSnapshot+1, 10))
	params.Set("result_overflow_mode", "throw")
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL.String(), nil)
	if err != nil {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("build ClickHouse Global Radar replay request: %w", err)
	}
	c.applyHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("ClickHouse Global Radar replay request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return services.GlobalRadarSnapshot{}, fmt.Errorf("ClickHouse Global Radar replay failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	limited := &io.LimitedReader{R: resp.Body, N: globalRadarGraphBodyLimit + 1}
	decoder := json.NewDecoder(limited)
	rows := make([]globalRadarGraphReadRow, 0)
	for {
		var row globalRadarGraphReadRow
		err := decoder.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return services.GlobalRadarSnapshot{}, fmt.Errorf("decode ClickHouse Global Radar replay row: %w", err)
		}
		if uint64(len(rows)) >= globalRadarReplayMaxRowsPerSnapshot {
			return services.GlobalRadarSnapshot{}, fmt.Errorf("ClickHouse Global Radar snapshot %s exceeded row limit %d", snapshotSHA256, globalRadarReplayMaxRowsPerSnapshot)
		}
		rows = append(rows, row)
	}
	if limited.N <= 0 {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("ClickHouse Global Radar replay response exceeded %d bytes", globalRadarGraphBodyLimit)
	}
	return rebuildGlobalRadarSnapshotForReplay(rows, snapshotSHA256, recordedAt)
}

func rebuildGlobalRadarSnapshotForReplay(rows []globalRadarGraphReadRow, snapshotSHA256 string, recordedAt time.Time) (services.GlobalRadarSnapshot, error) {
	if len(rows) == 0 || !hex64RE.MatchString(snapshotSHA256) || recordedAt.IsZero() {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay snapshot is incomplete")
	}
	observations := make([]services.GlobalRadarObservation, 0)
	relations := make([]services.GlobalRadarRelationEdge, 0)
	bridges := make([]services.GlobalRadarBridgeLink, 0)
	verdictRefs := make([]services.GlobalRadarVerdictReference, 0)

	for _, row := range rows {
		if strings.TrimSpace(row.SnapshotSHA256) != snapshotSHA256 || row.RecordedAtNanos != recordedAt.UnixNano() ||
			!globalRadarRecordTypeAllowed(row.RecordType) || !hex64RE.MatchString(strings.TrimSpace(row.PayloadSHA256)) {
			return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay row identity mismatch")
		}
		payload := json.RawMessage(row.PayloadJSON)
		if !json.Valid(payload) || jsonPayloadSHA256(payload) != strings.TrimSpace(row.PayloadSHA256) {
			return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay row payload hash mismatch")
		}
		switch row.RecordType {
		case "observation":
			var item services.GlobalRadarObservation
			if err := json.Unmarshal(payload, &item); err != nil || item.ObservationID != row.RecordID || item.Subject.ID != row.SubjectID {
				return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay observation mismatch")
			}
			observations = append(observations, item)
		case "relation":
			var item services.GlobalRadarRelationEdge
			if err := json.Unmarshal(payload, &item); err != nil || item.EdgeID != row.RecordID || item.Source.ID != row.SubjectID || item.Target.ID != row.TargetSubjectID {
				return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay relation mismatch")
			}
			relations = append(relations, item)
		case "bridge_link":
			var item services.GlobalRadarBridgeLink
			if err := json.Unmarshal(payload, &item); err != nil || item.LinkID != row.RecordID || item.SourceObservation.Subject.ID != row.SubjectID || item.DestinationObservation.Subject.ID != row.TargetSubjectID {
				return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay bridge-link mismatch")
			}
			bridges = append(bridges, item)
		case "verdict_reference":
			var item services.GlobalRadarVerdictReference
			if err := json.Unmarshal(payload, &item); err != nil || item.ReferenceID != row.RecordID || item.TargetSubjectID != row.SubjectID {
				return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay verdict-reference mismatch")
			}
			verdictRefs = append(verdictRefs, item)
		}
	}

	rebuilt, err := services.BuildGlobalRadarSnapshot(observations, relations, bridges, verdictRefs, recordedAt.UTC())
	if err != nil {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("rebuild Global Radar replay snapshot: %w", err)
	}
	body, err := json.Marshal(rebuilt)
	if err != nil {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("encode Global Radar replay snapshot: %w", err)
	}
	if jsonPayloadSHA256(body) != snapshotSHA256 {
		return services.GlobalRadarSnapshot{}, fmt.Errorf("Global Radar replay snapshot hash mismatch")
	}
	return rebuilt, nil
}
