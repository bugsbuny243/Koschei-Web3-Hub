package handlers

import (
	"fmt"
	"net/http"
	"time"
)

// SmartMoneyStream is retained for controlled integration. It remains
// disconnected from public routes until verified production enrichment exists.
func (h *Handler) SmartMoneyStream(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "websocket_upgrader_pending", "message": "Real data unavailable. Analysis could not be completed."})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "event: smart-money-error\ndata: {\"ok\":false,\"error\":\"real_data_unavailable\",\"message\":\"Real data unavailable. Analysis could not be completed.\",\"ts\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *Handler) SmartMoneySnapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "real_data_unavailable", "message": "Real data unavailable. Analysis could not be completed."})
}
