package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/radaradapter"
	"koschei/api/internal/services"
)

// TronWalletPermissions reports observed account permissions only. It does
// not issue a safety grade or claim block-anchored account state.
func (h *Handler) TronWalletPermissions(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" || len(address) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid Tron account address required"})
		return
	}
	adapter, err := radaradapter.NewTronMainnetWalletAdapter(nil, address)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Tron account address"})
		return
	}
	registry, err := services.NewUniversalAdapterRegistry(adapter)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Tron permission collector unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	result, err := registry.CollectTronWalletAddress(ctx, address)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Tron permission evidence unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"network":                       "tron-mainnet",
		"address":                       adapter.Subject().NativeID,
		"evidence_status":               "observed",
		"account_state_height_verified": false,
		"verdict":                       "not_assessed",
		"evidence":                      result.Evidence,
		"limitations":                   result.Limitations,
	})
}
