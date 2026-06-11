package handlers

import (
	"math"
	"net/http"
	"strings"
)

type liquidityRadarRequest struct {
	PoolAddress       string  `json:"pool_address"`
	TokenMint         string  `json:"token_mint"`
	ReserveDropPct    float64 `json:"reserve_drop_pct"`
	RemovedLiquidity  float64 `json:"removed_liquidity_usd"`
	BlockDelay        int     `json:"block_delay"`
	TelegramWebhook   string  `json:"telegram_webhook"`
	TwilioPhoneNumber string  `json:"twilio_phone_number"`
}

// LiquidityDrainAnalyze scores first-block rug-pull and liquidity drain risk.
// Twilio/SMS and Telegram delivery are represented as queued alert intents in
// phase 1; production workers will fan out signed webhooks from
// liquidity_drain_alerts.
func (h *Handler) LiquidityDrainAnalyze(w http.ResponseWriter, r *http.Request) {
	var req liquidityRadarRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if strings.TrimSpace(req.PoolAddress) == "" && strings.TrimSpace(req.TokenMint) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pool_or_token_required"})
		return
	}
	score := liquidityDrainScore(req)
	severity := "DÜŞÜK"
	if score >= 75 {
		severity = "KRİTİK"
	} else if score >= 45 {
		severity = "YÜKSEK"
	}
	lossPrevented := math.Round(req.RemovedLiquidity*float64(score)/100*100) / 100
	if h.DB != nil {
		_, _ = h.DB.ExecContext(r.Context(), `INSERT INTO liquidity_drain_alerts (pool_address, token_mint, severity, risk_score, removed_liquidity_usd, loss_prevented_usd, telegram_queued, sms_queued, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())`, req.PoolAddress, req.TokenMint, severity, score, req.RemovedLiquidity, lossPrevented, req.TelegramWebhook != "", req.TwilioPhoneNumber != "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "risk_score": score, "severity": severity, "liquidity_loss_prevented_usd": lossPrevented, "telegram_queued": req.TelegramWebhook != "", "sms_queued": req.TwilioPhoneNumber != "", "message": "Likidite boşaltma radarı alarmı üretildi."})
}

func liquidityDrainScore(req liquidityRadarRequest) int {
	score := 10
	if req.ReserveDropPct >= 20 {
		score += 30
	}
	if req.ReserveDropPct >= 50 {
		score += 30
	}
	if req.RemovedLiquidity >= 50_000 {
		score += 20
	}
	if req.BlockDelay <= 1 {
		score += 10
	}
	if score > 100 {
		return 100
	}
	return score
}
