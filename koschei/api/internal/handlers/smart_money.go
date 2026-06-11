package handlers

import (
	"fmt"
	"net/http"
	"time"
)

// SmartMoneyStream is the first technical foundation of the Institutional
// Smart Money Oracle. It is intentionally dependency-free: when a real
// WebSocket upgrader is added, this signature remains the route entrypoint and
// the same payload contract can be streamed to funds, market makers and paid
// B2B clients.
func (h *Handler) SmartMoneyStream(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "websocket_upgrader_pending", "message": "Kurumsal Smart Money WebSocket yükseltici sonraki fazda etkinleşecek."})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "event: smart-money\ndata: {\"type\":\"whale_cluster_snapshot\",\"wallet\":\"demo\",\"net_flow_usd\":1250000,\"confidence\":0.82,\"b2b_ready\":true,\"ts\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// SmartMoneySnapshot returns a B2B-friendly REST snapshot for customers that do
// not want a persistent stream. Production enrichment will read whale_clusters
// and cex_flows with customer-specific filters.
func (h *Handler) SmartMoneySnapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "module": "Institutional Smart Money Oracle", "enterprise_ready_api": true, "signals": []map[string]any{{"cluster": "solana_whales_alpha", "net_flow_usd": 1_250_000, "confidence": 0.82, "direction": "accumulation"}}})
}
