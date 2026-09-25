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
	"koschei/api/internal/radarevent"
)

const (
	globalRadarOwnerEventsSchemaVersion = "koschei.global-radar-events-response.v1"
	globalRadarOwnerEventDefaultWindow  = 24 * time.Hour
	globalRadarOwnerEventDefaultLimit   = uint64(200)
	globalRadarOwnerEventMaxLimit       = uint64(1000)
	globalRadarOwnerEventReadTimeout    = 20 * time.Second
)

type GlobalRadarEventReader interface {
	ReadGlobalRadarEvents(context.Context, koscheiclickhouse.GlobalRadarEventReadRequest) ([]koscheiclickhouse.GlobalRadarStoredEvent, error)
}

type globalRadarOwnerEventsQuery struct {
	Network   string    `json:"network"`
	SubjectID string    `json:"subject_id,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Since     time.Time `json:"since"`
	Until     time.Time `json:"until"`
	Limit     uint64    `json:"limit"`
}

type globalRadarOwnerEventsTruthBoundary struct {
	PersistedHistoricalEvidence bool   `json:"persisted_historical_evidence"`
	CurrentChainState           bool   `json:"current_chain_state"`
	PayloadHashReverified       bool   `json:"payload_hash_reverified"`
	EventDigestReverified       bool   `json:"event_digest_reverified"`
	RowPayloadIdentityMatched   bool   `json:"row_payload_identity_matched"`
	MissingEvidencePolicy       string `json:"missing_evidence_policy"`
}

type globalRadarOwnerEventsResponse struct {
	SchemaVersion string                                     `json:"schema_version"`
	Scope         string                                     `json:"scope"`
	GeneratedAt   time.Time                                  `json:"generated_at"`
	Query         globalRadarOwnerEventsQuery                `json:"query"`
	Count         int                                        `json:"count"`
	Events        []koscheiclickhouse.GlobalRadarStoredEvent `json:"events"`
	TruthBoundary globalRadarOwnerEventsTruthBoundary        `json:"truth_boundary"`
}

func ownerGlobalRadarEvents(reader GlobalRadarEventReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reader == nil {
			writeGlobalRadarOwnerError(w, http.StatusServiceUnavailable, "global_radar_event_reader_unavailable")
			return
		}
		query, err := parseGlobalRadarOwnerEventsQuery(r, time.Now().UTC())
		if err != nil {
			writeGlobalRadarOwnerError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), globalRadarOwnerEventReadTimeout)
		defer cancel()
		events, err := reader.ReadGlobalRadarEvents(ctx, koscheiclickhouse.GlobalRadarEventReadRequest{
			Network:   query.Network,
			SubjectID: query.SubjectID,
			Kind:      query.Kind,
			Since:     query.Since,
			Until:     query.Until,
			Limit:     query.Limit,
		})
		if err != nil {
			writeGlobalRadarOwnerError(w, http.StatusBadGateway, "global_radar_event_read_unavailable")
			return
		}
		if events == nil {
			events = []koscheiclickhouse.GlobalRadarStoredEvent{}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(globalRadarOwnerEventsResponse{
			SchemaVersion: globalRadarOwnerEventsSchemaVersion,
			Scope:         "persisted_canonical_events_not_current_chain_truth",
			GeneratedAt:   time.Now().UTC(),
			Query:         query,
			Count:         len(events),
			Events:        events,
			TruthBoundary: globalRadarOwnerEventsTruthBoundary{
				PersistedHistoricalEvidence: true,
				CurrentChainState:           false,
				PayloadHashReverified:       true,
				EventDigestReverified:       true,
				RowPayloadIdentityMatched:   true,
				MissingEvidencePolicy:       "unknown",
			},
		})
	}
}

func parseGlobalRadarOwnerEventsQuery(r *http.Request, now time.Time) (globalRadarOwnerEventsQuery, error) {
	values := r.URL.Query()
	network := strings.ToLower(strings.TrimSpace(values.Get("network")))
	if network == "" {
		return globalRadarOwnerEventsQuery{}, fmt.Errorf("network_required")
	}
	if _, ok := networktarget.LookupNetwork(network); !ok {
		return globalRadarOwnerEventsQuery{}, fmt.Errorf("unsupported_network")
	}

	until := now.UTC()
	since := until.Add(-globalRadarOwnerEventDefaultWindow)
	var err error
	if raw := strings.TrimSpace(values.Get("since")); raw != "" {
		since, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return globalRadarOwnerEventsQuery{}, fmt.Errorf("invalid_since")
		}
		since = since.UTC()
	}
	if raw := strings.TrimSpace(values.Get("until")); raw != "" {
		until, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return globalRadarOwnerEventsQuery{}, fmt.Errorf("invalid_until")
		}
		until = until.UTC()
	}
	if !since.Before(until) {
		return globalRadarOwnerEventsQuery{}, fmt.Errorf("invalid_time_window")
	}
	if until.Sub(since) > koscheiclickhouse.MaxGlobalRadarEventReadWindow {
		return globalRadarOwnerEventsQuery{}, fmt.Errorf("time_window_too_large")
	}

	limit := globalRadarOwnerEventDefaultLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 || parsed > globalRadarOwnerEventMaxLimit {
			return globalRadarOwnerEventsQuery{}, fmt.Errorf("invalid_limit")
		}
		limit = parsed
	}

	kind := strings.ToLower(strings.TrimSpace(values.Get("kind")))
	switch kind {
	case "",
		radarevent.KindBlock,
		radarevent.KindTransaction,
		radarevent.KindAccount,
		radarevent.KindAsset,
		radarevent.KindContract,
		radarevent.KindBridge,
		radarevent.KindLiquidity,
		radarevent.KindNetworkHealth:
	default:
		return globalRadarOwnerEventsQuery{}, fmt.Errorf("unsupported_event_kind")
	}

	return globalRadarOwnerEventsQuery{
		Network:   network,
		SubjectID: strings.TrimSpace(values.Get("subject_id")),
		Kind:      kind,
		Since:     since,
		Until:     until,
		Limit:     limit,
	}, nil
}
