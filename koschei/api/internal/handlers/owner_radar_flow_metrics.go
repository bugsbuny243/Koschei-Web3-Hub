package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	ownerRadarFlowMetricsSchemaVersion = "koschei.radar-flow-metrics.v1"
	ownerRadarFlowShortWindow          = 15 * time.Minute
	ownerRadarFlowLongWindow           = 24 * time.Hour
)

type ownerRadarFlowStage struct {
	Last15Minutes int64  `json:"last_15_minutes"`
	Last24Hours   int64  `json:"last_24_hours"`
	LastAt        string `json:"last_at,omitempty"`
}

type ownerRadarFlowQueue struct {
	Pending        int64 `json:"pending"`
	Processing     int64 `json:"processing"`
	StaleActive    int64 `json:"stale_active"`
	Failed15Min    int64 `json:"failed_last_15_minutes"`
	ExhaustedTotal int64 `json:"exhausted_total"`
}

type ownerRadarFlowRatios struct {
	RecognizedPerCollected15MinBP      int64 `json:"recognized_per_collected_15m_basis_points"`
	RecognizedPerCollected24hBP        int64 `json:"recognized_per_collected_24h_basis_points"`
	EnrichedPerCollected15MinBP        int64 `json:"enriched_per_collected_15m_basis_points"`
	EnrichedPerCollected24hBP          int64 `json:"enriched_per_collected_24h_basis_points"`
	ProcessedPerEnriched15MinBP        int64 `json:"processed_per_enriched_15m_basis_points"`
	ProcessedPerEnriched24hBP          int64 `json:"processed_per_enriched_24h_basis_points"`
	VerifiedVerdictsPerEnriched15MinBP int64 `json:"verified_verdicts_per_enriched_15m_basis_points"`
	VerifiedVerdictsPerEnriched24hBP   int64 `json:"verified_verdicts_per_enriched_24h_basis_points"`
}

type ownerRadarFlowFreshness struct {
	CollectedAgeSeconds int64 `json:"collected_age_seconds,omitempty"`
	EnrichedAgeSeconds  int64 `json:"enriched_age_seconds,omitempty"`
	ProcessedAgeSeconds int64 `json:"processed_age_seconds,omitempty"`
	VerdictAgeSeconds   int64 `json:"verdict_age_seconds,omitempty"`
	AlertAgeSeconds     int64 `json:"alert_age_seconds,omitempty"`
}

type ownerRadarFlowTruthBoundary struct {
	PersistedRowsOnly          bool   `json:"persisted_rows_only"`
	ChainWideCoverageClaim     bool   `json:"chain_wide_coverage_claim"`
	RecognizedMeansVerified    bool   `json:"recognized_means_verified"`
	EnrichedEvidenceDefinition string `json:"enriched_evidence_definition"`
	VerdictDefinition          string `json:"verdict_definition"`
	AlertDefinition            string `json:"alert_definition"`
	RatioUnit                  string `json:"ratio_unit"`
	RatioSemantics             string `json:"ratio_semantics"`
}

type ownerRadarFlowMetricsResponse struct {
	SchemaVersion    string                      `json:"schema_version"`
	GeneratedAt      time.Time                   `json:"generated_at"`
	Scope            string                      `json:"scope"`
	Collected        ownerRadarFlowStage         `json:"collected"`
	Recognized       ownerRadarFlowStage         `json:"recognized"`
	Enriched         ownerRadarFlowStage         `json:"enriched"`
	Processed        ownerRadarFlowStage         `json:"processed"`
	VerifiedVerdicts ownerRadarFlowStage         `json:"verified_verdicts"`
	DurableAlerts    ownerRadarFlowStage         `json:"durable_alerts"`
	Queue            ownerRadarFlowQueue         `json:"queue"`
	Ratios           ownerRadarFlowRatios        `json:"ratios"`
	Freshness        ownerRadarFlowFreshness     `json:"freshness"`
	EvidenceQuality  map[string]int64            `json:"evidence_quality_last_24_hours"`
	NetworkEvents    map[string]int64            `json:"network_events_last_24_hours"`
	AlertTypes       map[string]int64            `json:"alert_types_last_24_hours"`
	AlertSeverities  map[string]int64            `json:"alert_severities_last_24_hours"`
	TruthBoundary    ownerRadarFlowTruthBoundary `json:"truth_boundary"`
}

func (h *Handler) OwnerRadarFlowMetrics(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Radar flow metrics are unavailable")
		return
	}
	metrics, err := h.ownerRadarFlowMetrics(r.Context(), time.Now().UTC())
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Radar flow metrics could not be measured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "flow": metrics})
}

func (h *Handler) ownerRadarFlowMetrics(ctx context.Context, now time.Time) (ownerRadarFlowMetricsResponse, error) {
	var out ownerRadarFlowMetricsResponse
	out.SchemaVersion = ownerRadarFlowMetricsSchemaVersion
	out.GeneratedAt = now.UTC()
	out.Scope = "persisted_pipeline_rows_not_chain_wide_coverage"

	stage := func(query string, args ...any) (ownerRadarFlowStage, error) {
		var item ownerRadarFlowStage
		err := h.DBRead.QueryRowContext(ctx, query, args...).Scan(&item.Last15Minutes, &item.Last24Hours, &item.LastAt)
		return item, err
	}

	var err error
	out.Collected, err = stage(`
		SELECT
			count(*) FILTER (WHERE created_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE created_at > now() - interval '24 hours'),
			COALESCE(max(created_at)::text,'')
		FROM security_radar_stream_events
		WHERE created_at > now() - interval '24 hours'
	`)
	if err != nil {
		return out, err
	}
	out.Recognized, err = stage(`
		SELECT
			count(*) FILTER (WHERE created_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE created_at > now() - interval '24 hours'),
			COALESCE(max(created_at)::text,'')
		FROM security_radar_stream_events
		WHERE created_at > now() - interval '24 hours' AND module_id <> 'unknown'
	`)
	if err != nil {
		return out, err
	}
	out.Enriched, err = stage(`
		SELECT
			count(*) FILTER (WHERE created_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE created_at > now() - interval '24 hours'),
			COALESCE(max(created_at)::text,'')
		FROM security_radar_stream_events
		WHERE created_at > now() - interval '24 hours' AND evidence_quality='transaction_enriched_mint'
	`)
	if err != nil {
		return out, err
	}
	out.Processed, err = stage(`
		SELECT
			count(*) FILTER (WHERE processed_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE processed_at > now() - interval '24 hours'),
			COALESCE(max(processed_at)::text,'')
		FROM arvis_stream_processing
		WHERE processed_at > now() - interval '24 hours' AND status='completed'
	`)
	if err != nil {
		return out, err
	}
	verifiedSQL := radarVerifiedEvidenceSQL()
	out.VerifiedVerdicts, err = stage(`
		SELECT
			count(*) FILTER (WHERE created_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE created_at > now() - interval '24 hours'),
			COALESCE(max(created_at)::text,'')
		FROM security_radar_verdicts
		WHERE created_at > now() - interval '24 hours' AND module_id='final_verdict_engine' AND signed=true AND `+verifiedSQL)
	if err != nil {
		return out, err
	}
	out.DurableAlerts, err = stage(`
		SELECT
			count(*) FILTER (WHERE created_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE created_at > now() - interval '24 hours'),
			COALESCE(max(created_at)::text,'')
		FROM security_alert_events
		WHERE created_at > now() - interval '24 hours'
	`)
	if err != nil {
		return out, err
	}

	if err := h.DBRead.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE status='pending'),
			count(*) FILTER (WHERE status='processing'),
			count(*) FILTER (WHERE status='processing' AND updated_at < now() - interval '5 minutes'),
			count(*) FILTER (WHERE status='failed' AND updated_at > now() - interval '15 minutes'),
			count(*) FILTER (WHERE status='exhausted' OR (status='failed' AND attempts >= 3))
		FROM arvis_stream_processing
	`).Scan(
		&out.Queue.Pending,
		&out.Queue.Processing,
		&out.Queue.StaleActive,
		&out.Queue.Failed15Min,
		&out.Queue.ExhaustedTotal,
	); err != nil {
		return out, err
	}

	out.EvidenceQuality, err = h.ownerRadarFlowBreakdown(ctx, `
		SELECT evidence_quality, count(*)
		FROM security_radar_stream_events
		WHERE created_at > now() - interval '24 hours'
		GROUP BY evidence_quality
		ORDER BY count(*) DESC
		LIMIT 32
	`)
	if err != nil {
		return out, err
	}
	out.NetworkEvents, err = h.ownerRadarFlowBreakdown(ctx, `
		SELECT network, count(*)
		FROM security_radar_stream_events
		WHERE created_at > now() - interval '24 hours'
		GROUP BY network
		ORDER BY count(*) DESC
		LIMIT 32
	`)
	if err != nil {
		return out, err
	}
	out.AlertTypes, err = h.ownerRadarFlowBreakdown(ctx, `
		SELECT event_type, count(*)
		FROM security_alert_events
		WHERE created_at > now() - interval '24 hours'
		GROUP BY event_type
		ORDER BY count(*) DESC
		LIMIT 16
	`)
	if err != nil {
		return out, err
	}
	out.AlertSeverities, err = h.ownerRadarFlowBreakdown(ctx, `
		SELECT severity, count(*)
		FROM security_alert_events
		WHERE created_at > now() - interval '24 hours'
		GROUP BY severity
		ORDER BY count(*) DESC
		LIMIT 8
	`)
	if err != nil {
		return out, err
	}

	out.Ratios = ownerRadarFlowRatios{
		RecognizedPerCollected15MinBP:      radarFlowBasisPoints(out.Recognized.Last15Minutes, out.Collected.Last15Minutes),
		RecognizedPerCollected24hBP:        radarFlowBasisPoints(out.Recognized.Last24Hours, out.Collected.Last24Hours),
		EnrichedPerCollected15MinBP:        radarFlowBasisPoints(out.Enriched.Last15Minutes, out.Collected.Last15Minutes),
		EnrichedPerCollected24hBP:          radarFlowBasisPoints(out.Enriched.Last24Hours, out.Collected.Last24Hours),
		ProcessedPerEnriched15MinBP:        radarFlowBasisPoints(out.Processed.Last15Minutes, out.Enriched.Last15Minutes),
		ProcessedPerEnriched24hBP:          radarFlowBasisPoints(out.Processed.Last24Hours, out.Enriched.Last24Hours),
		VerifiedVerdictsPerEnriched15MinBP: radarFlowBasisPoints(out.VerifiedVerdicts.Last15Minutes, out.Enriched.Last15Minutes),
		VerifiedVerdictsPerEnriched24hBP:   radarFlowBasisPoints(out.VerifiedVerdicts.Last24Hours, out.Enriched.Last24Hours),
	}
	out.Freshness = ownerRadarFlowFreshness{
		CollectedAgeSeconds: radarFlowAgeSeconds(now, out.Collected.LastAt),
		EnrichedAgeSeconds:  radarFlowAgeSeconds(now, out.Enriched.LastAt),
		ProcessedAgeSeconds: radarFlowAgeSeconds(now, out.Processed.LastAt),
		VerdictAgeSeconds:   radarFlowAgeSeconds(now, out.VerifiedVerdicts.LastAt),
		AlertAgeSeconds:     radarFlowAgeSeconds(now, out.DurableAlerts.LastAt),
	}
	out.TruthBoundary = ownerRadarFlowTruthBoundary{
		PersistedRowsOnly:          true,
		ChainWideCoverageClaim:     false,
		RecognizedMeansVerified:    false,
		EnrichedEvidenceDefinition: "security_radar_stream_events.evidence_quality=transaction_enriched_mint",
		VerdictDefinition:          "final_verdict_engine AND signed=true AND verified_evidence=true",
		AlertDefinition:            "new durable security_alert_events rows; repeat deduped occurrences may update the same row instead of creating another row",
		RatioUnit:                  "basis_points_10000_equals_100_percent",
		RatioSemantics:             "windowed throughput ratios are not cohort conversion rates and may exceed 100 percent when backlog is processed",
	}
	return out, nil
}

func (h *Handler) ownerRadarFlowBreakdown(ctx context.Context, query string) (map[string]int64, error) {
	rows, err := h.DBRead.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		key = strings.TrimSpace(key)
		if key == "" {
			key = "unknown"
		}
		out[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func radarFlowBasisPoints(numerator, denominator int64) int64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (numerator * 10000) / denominator
}

func radarFlowAgeSeconds(now time.Time, value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed := metricTime(map[string]any{"value": value}, "value")
	if parsed.IsZero() {
		return 0
	}
	age := now.UTC().Sub(parsed.UTC())
	if age <= 0 {
		return 0
	}
	return int64(age / time.Second)
}
