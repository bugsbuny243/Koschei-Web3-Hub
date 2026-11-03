package handlers

import (
	"net/http"

	"koschei/api/internal/services"
)

// OwnerTronFlowRadar reports the in-memory Tron Flow Radar: progress, the
// addresses currently followed from seed wallets and recent findings. It reads
// process memory only; nothing here comes from or goes to the database.
func (h *Handler) OwnerTronFlowRadar(w http.ResponseWriter, r *http.Request) {
	radar := services.ActiveTronFlowRadar()
	if radar == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "enabled": false, "storage": "in_memory_only",
			"reason": "Tron Flow Radar is off. Set KOSCHEI_TRON_FLOW_RADAR_ENABLED=1 (TRONGRID_API_KEY required) on a process that runs background workers.",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": true, "radar": radar.Snapshot()})
}
