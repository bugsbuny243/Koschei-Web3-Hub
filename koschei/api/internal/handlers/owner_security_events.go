package handlers

import (
	"net/http"
	"strconv"

	"koschei/api/internal/services"
)

func (h *Handler) OwnerSecurityEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	items, err := services.LatestSecurityAuditEvents(r.Context(), h.DBRead, limit)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "events": []services.SecurityAuditEvent{}, "error": "Security events unavailable."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "events": items})
}
