package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const ownerGlobalCampaignReadTimeout = 10 * time.Second

type ownerGlobalCampaignsResponse struct {
	SchemaVersion string                    `json:"schema_version"`
	Scope         string                    `json:"scope"`
	GeneratedAt   time.Time                 `json:"generated_at"`
	Count         int                       `json:"count"`
	Campaigns     []services.GlobalCampaign `json:"campaigns"`
	Authority     map[string]bool           `json:"authority"`
}

func (h *Handler) OwnerGlobalCampaigns(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "database_unavailable"})
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > services.GlobalCampaignListMaxLimit {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_limit"})
			return
		}
		limit = parsed
	}
	ctx, cancel := context.WithTimeout(r.Context(), ownerGlobalCampaignReadTimeout)
	defer cancel()
	campaigns, err := services.ListCurrentGlobalCampaigns(ctx, h.DB, limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "global_campaign_read_unavailable"})
		return
	}
	if campaigns == nil {
		campaigns = []services.GlobalCampaign{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(ownerGlobalCampaignsResponse{
		SchemaVersion: "koschei.owner-global-campaigns.v1",
		Scope:         "persisted_campaign_correlation_memory",
		GeneratedAt:   time.Now().UTC(),
		Count:         len(campaigns),
		Campaigns:     campaigns,
		Authority: map[string]bool{
			"verdict": false, "grade": false, "containment": false, "execution": false,
		},
	})
}
