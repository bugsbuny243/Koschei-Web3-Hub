package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"koschei/api/internal/services"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.DBPingError(); err != nil {
		log.Printf("health check database ping failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "error",
			"database": "unavailable",
			"details":  err.Error(),
			"arvis": map[string]any{
				"pipeline_status": "database_unavailable",
			},
		})
		return
	}

	stats := h.securityRadarStreamStats(r.Context())
	arvis := map[string]any{
		"pipeline_status":          stats["pipeline_status"],
		"architecture_arm_count":   stats["architecture_arm_count"],
		"runtime_engines":          stats["runtime_engines"],
		"raw_stream_events":        stats["raw_stream_events"],
		"recognized_events":        stats["recognized_events"],
		"enriched_mints":           stats["enriched_mints"],
		"visible_verdicts":         stats["visible_verdicts"],
		"processing_active":        stats["processing_active"],
		"processing_completed":     stats["processing_completed"],
		"processing_insufficient":  stats["processing_insufficient"],
		"processing_failed":        stats["processing_failed"],
		"last_stream_event_at":     stats["last_stream_event_at"],
		"last_processed_at":        stats["last_processed_at"],
		"runtime_window_minutes":   stats["runtime_window_minutes"],
		"sources":                  h.arvisSourceHealth(r.Context()),
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"database": "connected",
		"service":  "koschei-web3",
		"arvis":    arvis,
	})
}

func (h *Handler) arvisSourceHealth(ctx context.Context) map[string]any {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		return map[string]any{}
	}
	return map[string]any{
		"pump":    arvisModuleSourceHealth(ctx, db, services.ModulePumpSybilRadar),
		"raydium": arvisModuleSourceHealth(ctx, db, services.ModuleRaydiumPoolGuardian),
	}
}

func arvisModuleSourceHealth(ctx context.Context, db *sql.DB, moduleID string) map[string]any {
	out := map[string]any{
		"module_id": moduleID,
		"events":    int64(0),
		"recent":    int64(0),
		"enriched":  int64(0),
		"last_event_at": "",
	}
	if db == nil || moduleID == "" {
		return out
	}
	count := func(key, query string) {
		var value int64
		if err := db.QueryRowContext(ctx, query, moduleID).Scan(&value); err == nil {
			out[key] = value
		}
	}
	count("events", `SELECT count(*) FROM security_radar_stream_events WHERE module_id=$1`)
	count("recent", `SELECT count(*) FROM security_radar_stream_events WHERE module_id=$1 AND created_at > now() - interval '15 minutes'`)
	count("enriched", `SELECT count(*) FROM security_radar_stream_events WHERE module_id=$1 AND evidence_quality='transaction_enriched_mint'`)
	var lastEvent string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_stream_events WHERE module_id=$1`, moduleID).Scan(&lastEvent); err == nil {
		out["last_event_at"] = lastEvent
	}
	return out
}
