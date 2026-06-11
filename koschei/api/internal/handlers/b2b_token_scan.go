package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type b2bTokenScanRequest struct {
	Mint      string `json:"mint"`
	Address   string `json:"address"`
	Network   string `json:"network"`
	IncludeAI bool   `json:"include_ai"`
}

// B2BTokenScan godoc
// @Summary Token risk analizi
// @Description API anahtarı ile Solana token risk analizi başlatır.
// @Tags B2B API
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body b2bTokenScanRequest true "Token analiz isteği"
// @Success 202 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 402 {object} map[string]string
// @Failure 429 {object} map[string]string
// @Router /api/v1/scan/token [post]
func (h *Handler) B2BTokenScan(w http.ResponseWriter, r *http.Request) {
	principal, ok := apiPrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req b2bTokenScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	mint := strings.TrimSpace(req.Mint)
	if mint == "" {
		mint = strings.TrimSpace(req.Address)
	}
	if mint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mint_required"})
		return
	}
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}
	requestID, err := newUUID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "request_id_failed"})
		return
	}
	const cost = 1
	if err := h.reserveAPICredits(r.Context(), principal, "/api/v1/scan/token", requestID, cost); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	go h.runB2BTokenScan(requestID, req.Network, mint)
	writeJSON(w, http.StatusAccepted, map[string]any{"request_id": requestID, "status": "queued", "cost_credits": cost, "message": "Token analizi kuyruğa alındı."})
}

func (h *Handler) runB2BTokenScan(requestID, network, mint string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := h.tokenService().ScanToken(ctx, network, mint)
	if err != nil {
		h.refundAPICredits(ctx, requestID, "rpc_scan_failed")
		return
	}
	payload, _ := json.Marshal(result)
	_, _ = h.DB.ExecContext(ctx, `UPDATE api_usage_events SET metadata=$1::jsonb WHERE request_id=$2`, string(payload), requestID)
	h.finalizeAPIUsage(ctx, requestID, int(time.Since(start).Milliseconds()))
}
