package handlers

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"koschei/api/internal/services"
)

const publicRadarTelemetryTTL = 15 * time.Second

var publicRadarTelemetryInitMu sync.Mutex

// A per-handler cache coalesces concurrent public polls. It contains only
// telemetry: current verdicts and their visibility are read on every request.
type publicRadarTelemetryCache struct {
	mu       sync.Mutex
	db       *sql.DB
	snapshot map[string]any
	expires  time.Time
	loading  chan struct{}
}

func unavailablePublicRadarTelemetry() map[string]any {
	return map[string]any{
		"pipeline_status": "unknown", "telemetry_status": "unavailable",
		"source_health": map[string]any{},
	}
}

func (h *Handler) publicRadarLiveTelemetry(ctx context.Context, db *sql.DB) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	// Handler is copied during route setup; keep the synchronized cache behind
	// a pointer and initialize it under a separate lock for concurrent first use.
	publicRadarTelemetryInitMu.Lock()
	if h.publicRadarTelemetry == nil {
		h.publicRadarTelemetry = &publicRadarTelemetryCache{}
	}
	c := h.publicRadarTelemetry
	publicRadarTelemetryInitMu.Unlock()
	c.mu.Lock()
	if c.db == db && c.snapshot != nil && time.Now().Before(c.expires) {
		snapshot := c.snapshot
		c.mu.Unlock()
		return snapshot
	}
	if done := c.loading; done != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return unavailablePublicRadarTelemetry()
		case <-done:
			c.mu.Lock()
			snapshot := c.snapshot
			matches := c.db == db
			c.mu.Unlock()
			if matches && snapshot != nil {
				return snapshot
			}
			return unavailablePublicRadarTelemetry()
		}
	}
	c.loading = make(chan struct{})
	c.mu.Unlock()

	snapshot := loadPublicRadarTelemetry(ctx, db)
	ttl := publicRadarTelemetryTTL
	if snapshot["telemetry_status"] != "available" {
		ttl = 2 * time.Second
	}
	c.mu.Lock()
	c.db, c.snapshot, c.expires = db, snapshot, time.Now().Add(ttl)
	close(c.loading)
	c.loading = nil
	c.mu.Unlock()
	return snapshot
}

// Stream reads are index-backed latest-row probes. Lifetime exact counts of
// multi-million-row journals belong in an asynchronous operator projection,
// not the public request path. A planner estimate is never labeled an exact
// count, and a failed query never becomes a zero or a healthy status.
func loadPublicRadarTelemetry(ctx context.Context, db *sql.DB) map[string]any {
	metrics := unavailablePublicRadarTelemetry()
	metrics["observed_at"] = time.Now().UTC()
	if db == nil {
		return metrics
	}
	var lastEvent sql.NullTime
	var estimate sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT
			(SELECT created_at FROM security_radar_stream_events ORDER BY created_at DESC LIMIT 1) AS last_stream_event_at,
			(SELECT CASE WHEN reltuples >= 0 THEN reltuples::bigint END
			 FROM pg_class WHERE oid=to_regclass('security_radar_stream_events')) AS raw_stream_events_estimate
	`).Scan(&lastEvent, &estimate)
	if err != nil {
		return metrics
	}
	if lastEvent.Valid {
		metrics["last_stream_event_at"] = lastEvent.Time.UTC().Format(time.RFC3339Nano)
	}
	if estimate.Valid {
		metrics["raw_stream_events_estimate"] = estimate.Int64
	}

	var active, completed, failed, stale, completedRecent, failedRecent int64
	var lastProcessed sql.NullTime
	if err := db.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE status='processing'),
			count(*) FILTER (WHERE status='completed'),
			count(*) FILTER (WHERE status='failed'),
			count(*) FILTER (WHERE status='processing' AND updated_at < now()-interval '5 minutes'),
			count(*) FILTER (WHERE status='completed' AND processed_at > now()-interval '15 minutes'),
			count(*) FILTER (WHERE status='failed' AND updated_at > now()-interval '15 minutes'),
			max(processed_at)
		FROM arvis_stream_processing
	`).Scan(&active, &completed, &failed, &stale, &completedRecent, &failedRecent, &lastProcessed); err != nil {
		metrics["telemetry_status"] = "partial"
		return metrics
	}
	metrics["processing_active"] = active
	metrics["processing_completed"] = completed
	metrics["processing_failed"] = failed
	metrics["processing_stale_active"] = stale
	metrics["processing_completed_recent"] = completedRecent
	metrics["processing_failed_recent"] = failedRecent
	if lastProcessed.Valid {
		metrics["last_processed_at"] = lastProcessed.Time.UTC().Format(time.RFC3339Nano)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT sources.module_id, latest.created_at, enriched.created_at
		FROM (VALUES ($1::text), ($2::text)) AS sources(module_id)
		LEFT JOIN LATERAL (
			SELECT created_at FROM security_radar_stream_events
			WHERE module_id=sources.module_id ORDER BY created_at DESC LIMIT 1
		) latest ON true
		LEFT JOIN LATERAL (
			SELECT created_at FROM security_radar_stream_events
			WHERE module_id=sources.module_id AND evidence_quality='transaction_enriched_mint'
			ORDER BY created_at DESC LIMIT 1
		) enriched ON true
	`, services.ModulePumpSybilRadar, services.ModuleRaydiumPoolGuardian)
	if err != nil {
		metrics["telemetry_status"] = "partial"
		return metrics
	}
	defer rows.Close()
	sources := map[string]any{}
	enrichedObserved := false
	for rows.Next() {
		var module string
		var latest, enriched sql.NullTime
		if err := rows.Scan(&module, &latest, &enriched); err != nil {
			metrics["telemetry_status"] = "partial"
			return metrics
		}
		source := map[string]any{"module_id": module, "events": nil, "recent": nil, "enriched": nil}
		if latest.Valid {
			source["last_event_at"] = latest.Time.UTC().Format(time.RFC3339Nano)
		}
		if enriched.Valid {
			enrichedObserved = true
			source["last_enriched_at"] = enriched.Time.UTC().Format(time.RFC3339Nano)
		}
		name := "pump"
		if module == services.ModuleRaydiumPoolGuardian {
			name = "raydium"
		}
		sources[name] = source
	}
	if rows.Err() != nil {
		metrics["telemetry_status"] = "partial"
		return metrics
	}
	metrics["source_health"] = sources
	metrics["telemetry_status"] = "available"
	status := "waiting_for_processing"
	switch {
	case active > 0 && stale == 0:
		status = "processing"
	case stale > 0 || failedRecent > 0:
		status = "degraded"
	case !lastEvent.Valid:
		status = "waiting_for_stream"
	case time.Since(lastEvent.Time) > 10*time.Minute:
		status = "stale"
	case !enrichedObserved && completed == 0:
		status = "waiting_for_enriched_targets"
	case completedRecent > 0 || (lastProcessed.Valid && time.Since(lastProcessed.Time) <= 15*time.Minute):
		status = "healthy"
	}
	metrics["pipeline_status"] = status
	return metrics
}
