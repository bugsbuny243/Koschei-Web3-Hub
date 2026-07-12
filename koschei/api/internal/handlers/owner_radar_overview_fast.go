package handlers

import (
	"context"
	"net/http"
	"time"

	"koschei/api/internal/services"
)

// OwnerRadarOverviewFast keeps the owner workspace responsive even when the
// historical radar tables are large. Each read is bounded and failures return
// partial production data instead of holding the whole page open.
func (h *Handler) OwnerRadarOverviewFast(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	items := []services.SecurityRadarVerdictRecord{}
	sources := []services.SecurityRadarSource{}
	highVolumePump := []services.PumpHighVolumeOwnerItem{}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if db != nil {
		store := services.NewSecurityRadarStore(db)
		if loaded, err := store.LatestVerdicts(ctx, 40); err == nil {
			items = loaded
		}
		if loaded, err := store.ListSources(ctx); err == nil {
			sources = loaded
		}
		if loaded, err := store.LatestPumpHighVolumeReportsExact(ctx, 50); err == nil {
			highVolumePump = loaded
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "generated_at": time.Now().UTC(), "items": items,
		"high_volume_pump": highVolumePump,
		"sources":          sources, "pipeline": h.securityRadarStreamStats(ctx),
	})
}
