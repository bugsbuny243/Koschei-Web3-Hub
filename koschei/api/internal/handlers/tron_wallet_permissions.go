package handlers

import (
	"context"
	"koschei/api/internal/radaradapter"
	"koschei/api/internal/services"
	"log"
	"net/http"
	"strings"
	"time"
)

const tronCollectorProvider = "https://api.trongrid.io/wallet/getaccount"

// TronWalletPermissions reports observations, never a safety grade or a claim
// that getaccount state is anchored to the separately fetched head height.
func (h *Handler) TronWalletPermissions(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	status, reason, rows := "rejected", "address_missing_or_too_long", 0
	var providerLatency int64
	defer func() {
		log.Printf("tron wallet permissions collector status=%s reason=%s rows=%d latency_ms=%d provider_latency_ms=%d provider=%s", status, reason, rows, time.Since(started).Milliseconds(), providerLatency, tronCollectorProvider)
	}()
	fail := func(code int, message string) {
		writeJSON(w, code, map[string]any{"ok": false, "error": message, "collector_status": status, "provider": tronCollectorProvider, "provider_latency_ms": providerLatency, "evidence_row_count": 0})
	}
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" || len(address) > 128 {
		fail(http.StatusBadRequest, "valid Tron account address required")
		return
	}
	adapter, err := radaradapter.NewTronMainnetWalletAdapter(nil, address)
	if err != nil {
		reason = "address_invalid"
		fail(http.StatusBadRequest, "invalid Tron account address")
		return
	}
	registry, err := services.NewUniversalAdapterRegistry(adapter)
	if err != nil {
		status, reason = "unavailable", "registry_construction_failed"
		fail(http.StatusServiceUnavailable, "Tron permission collector unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	collectionStarted := time.Now()
	result, err := registry.CollectTronWalletAddress(ctx, address)
	providerLatency = time.Since(collectionStarted).Milliseconds()
	if err != nil {
		status, reason = "provider_unavailable", "provider_collection_failed"
		fail(http.StatusServiceUnavailable, "Tron permission evidence unavailable")
		return
	}
	reasons := make([]string, 0, len(result.Limitations)+1)
	for _, limitation := range result.Limitations {
		reasons = append(reasons, limitation.Code+": "+limitation.Reason)
	}
	reasons = append(reasons, "TRON_PERMISSIONS_ARE_OBSERVED_ONLY: an observed permission set is not a safety grade.")
	status, reason, rows = "collected", "ok", len(result.Evidence)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "verdict": "withhold", "unmet_evidence_reasons": reasons,
		"evidence": result.Evidence, "network": "tron-mainnet", "address": adapter.Subject().NativeID,
		"evidence_status": "observed", "grade_issued": false, "account_state_height_verified": false,
		"collector_status": status, "provider": tronCollectorProvider, "provider_latency_ms": providerLatency, "evidence_row_count": rows,
		"limitations": result.Limitations,
	})
}
