package handlers

import (
	"log"
	"net/http"
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
		"pipeline_status":        stats["pipeline_status"],
		"architecture_arm_count": stats["architecture_arm_count"],
		"runtime_engines":        stats["runtime_engines"],
		"raw_stream_events":      stats["raw_stream_events"],
		"recognized_events":      stats["recognized_events"],
		"enriched_mints":         stats["enriched_mints"],
		"visible_verdicts":       stats["visible_verdicts"],
		"processing_active":      stats["processing_active"],
		"processing_completed":   stats["processing_completed"],
		"processing_insufficient": stats["processing_insufficient"],
		"processing_failed":      stats["processing_failed"],
		"last_stream_event_at":   stats["last_stream_event_at"],
		"last_processed_at":      stats["last_processed_at"],
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"database": "connected",
		"service":  "koschei-web3",
		"arvis":    arvis,
	})
}
