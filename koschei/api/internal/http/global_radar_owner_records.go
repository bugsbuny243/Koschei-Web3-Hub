package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
	"koschei/api/internal/networktarget"
)

const (
	globalRadarOwnerRecordsSchemaVersion = "koschei.global-radar-records-response.v1"
	globalRadarOwnerDefaultWindow        = 24 * time.Hour
	globalRadarOwnerDefaultLimit         = uint64(200)
	globalRadarOwnerMaxLimit             = uint64(1000)
	globalRadarOwnerReadTimeout          = 20 * time.Second
)

type GlobalRadarGraphReader interface {
	ReadGlobalRadarGraph(context.Context, koscheiclickhouse.GlobalRadarGraphReadRequest) ([]koscheiclickhouse.GlobalRadarStoredRecord, error)
}

type globalRadarOwnerRecordsQuery struct {
	Network    string    `json:"network"`
	SubjectID  string    `json:"subject_id,omitempty"`
	RecordType string    `json:"record_type,omitempty"`
	Since      time.Time `json:"since"`
	Until      time.Time `json:"until"`
	Limit      uint64    `json:"limit"`
}

type globalRadarOwnerRecordsTruthBoundary struct {
	PersistedHistoricalEvidence bool   `json:"persisted_historical_evidence"`
	CurrentChainState           bool   `json:"current_chain_state"`
	PayloadHashReverified       bool   `json:"payload_hash_reverified"`
	MissingEvidencePolicy       string `json:"missing_evidence_policy"`
}

type globalRadarOwnerRecordsResponse struct {
	SchemaVersion string                                      `json:"schema_version"`
	Scope         string                                      `json:"scope"`
	GeneratedAt   time.Time                                   `json:"generated_at"`
	Query         globalRadarOwnerRecordsQuery                `json:"query"`
	Count         int                                         `json:"count"`
	Records       []koscheiclickhouse.GlobalRadarStoredRecord `json:"records"`
	TruthBoundary globalRadarOwnerRecordsTruthBoundary        `json:"truth_boundary"`
}

func ownerGlobalRadarGraphRecords(reader GlobalRadarGraphReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reader == nil {
			writeGlobalRadarOwnerError(w, http.StatusServiceUnavailable, "global_radar_graph_reader_unavailable")
			return
		}

		query, err := parseGlobalRadarOwnerRecordsQuery(r, time.Now().UTC())
		if err != nil {
			writeGlobalRadarOwnerError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), globalRadarOwnerReadTimeout)
		defer cancel()
		records, err := reader.ReadGlobalRadarGraph(ctx, koscheiclickhouse.GlobalRadarGraphReadRequest{
			Network:    query.Network,
			SubjectID:  query.SubjectID,
			RecordType: query.RecordType,
			Since:      query.Since,
			Until:      query.Until,
			Limit:      query.Limit,
		})
		if err != nil {
			writeGlobalRadarOwnerError(w, http.StatusBadGateway, "global_radar_graph_read_unavailable")
			return
		}
		if records == nil {
			records = []koscheiclickhouse.GlobalRadarStoredRecord{}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(globalRadarOwnerRecordsResponse{
			SchemaVersion: globalRadarOwnerRecordsSchemaVersion,
			Scope:         "persisted_historical_graph_records_not_current_chain_truth",
			GeneratedAt:   time.Now().UTC(),
			Query:         query,
			Count:         len(records),
			Records:       records,
			TruthBoundary: globalRadarOwnerRecordsTruthBoundary{
				PersistedHistoricalEvidence: true,
				CurrentChainState:           false,
				PayloadHashReverified:       true,
				MissingEvidencePolicy:       "unknown",
			},
		})
	}
}

func parseGlobalRadarOwnerRecordsQuery(r *http.Request, now time.Time) (globalRadarOwnerRecordsQuery, error) {
	values := r.URL.Query()
	network := strings.ToLower(strings.TrimSpace(values.Get("network")))
	if network == "" {
		return globalRadarOwnerRecordsQuery{}, fmt.Errorf("network_required")
	}
	if _, ok := networktarget.LookupNetwork(network); !ok {
		return globalRadarOwnerRecordsQuery{}, fmt.Errorf("unsupported_network")
	}

	until := now.UTC()
	since := until.Add(-globalRadarOwnerDefaultWindow)
	var err error
	if raw := strings.TrimSpace(values.Get("since")); raw != "" {
		since, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return globalRadarOwnerRecordsQuery{}, fmt.Errorf("invalid_since")
		}
		since = since.UTC()
	}
	if raw := strings.TrimSpace(values.Get("until")); raw != "" {
		until, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return globalRadarOwnerRecordsQuery{}, fmt.Errorf("invalid_until")
		}
		until = until.UTC()
	}
	if !since.Before(until) {
		return globalRadarOwnerRecordsQuery{}, fmt.Errorf("invalid_time_window")
	}
	if until.Sub(since) > koscheiclickhouse.MaxGlobalRadarGraphReadWindow {
		return globalRadarOwnerRecordsQuery{}, fmt.Errorf("time_window_too_large")
	}

	limit := globalRadarOwnerDefaultLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 || parsed > globalRadarOwnerMaxLimit {
			return globalRadarOwnerRecordsQuery{}, fmt.Errorf("invalid_limit")
		}
		limit = parsed
	}

	recordType := strings.ToLower(strings.TrimSpace(values.Get("record_type")))
	switch recordType {
	case "", "observation", "relation", "bridge_link", "verdict_reference":
	default:
		return globalRadarOwnerRecordsQuery{}, fmt.Errorf("unsupported_record_type")
	}

	return globalRadarOwnerRecordsQuery{
		Network:    network,
		SubjectID:  strings.TrimSpace(values.Get("subject_id")),
		RecordType: recordType,
		Since:      since,
		Until:      until,
		Limit:      limit,
	}, nil
}

func writeGlobalRadarOwnerError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":               code,
		"current_chain_state": false,
	})
}
