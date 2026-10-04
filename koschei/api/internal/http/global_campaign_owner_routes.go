package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"koschei/api/internal/handlers"
	"koschei/api/internal/services"
)

func registerGlobalCampaignOwnerRoutes(mux *http.ServeMux, h *handlers.Handler, staticDir string) {
	mux.HandleFunc("/api/owner/campaigns", ownerOnly(h, method("GET", ownerGlobalCampaigns(h))))
	mux.HandleFunc("/owner/campaigns", ownerOnly(h, method("GET", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filepath.Join(staticDir, "campaigns.html"))
	})))
}

func ownerGlobalCampaigns(h *handlers.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		enabled := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED")) == "1"
		if h.DB == nil {
			writeSecurityJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "campaign_store_unavailable"})
			return
		}
		status, snapshots, err := services.ReadGlobalCampaignRuntime(ctx, h.DB, enabled)
		if err != nil {
			writeSecurityJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "campaign_runtime_read_unavailable"})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(struct {
			Version   string                                   `json:"version"`
			Runtime   services.GlobalCampaignRuntimeStatus     `json:"runtime"`
			Campaigns []services.GlobalCampaignRuntimeSnapshot `json:"campaigns"`
		}{services.GlobalCampaignRuntimeVersion, status, snapshots})
	}
}
