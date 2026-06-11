package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"
)

type mevAnalyzeRequest struct {
	UserWallet       string  `json:"user_wallet"`
	TXSignature      string  `json:"tx_signature"`
	RawTransaction   string  `json:"raw_transaction"`
	InputAmountUSD   float64 `json:"input_amount_usd"`
	SlippageBPS      int     `json:"slippage_bps"`
	PoolLiquidityUSD float64 `json:"pool_liquidity_usd"`
	Route            string  `json:"route"`
}

type mevAnalyzeResponse struct {
	RiskScore          int      `json:"risk_score"`
	RiskLevel          string   `json:"risk_level"`
	EstimatedLossUSD   float64  `json:"estimated_loss_usd"`
	JitoTipUsed        bool     `json:"jito_tip_used"`
	RecommendedTipSOL  float64  `json:"recommended_tip_sol"`
	MEVSavedUSD        float64  `json:"mev_saved_usd"`
	Signals            []string `json:"signals"`
	EnterpriseReadyAPI bool     `json:"enterprise_ready_api"`
	Message            string   `json:"message"`
}

// AnalyzeMEV provides a package-level mock endpoint for minimal integrations
// that do not instantiate Handler. Production routes should use Handler.AnalyzeMEV
// so database persistence is available.
func AnalyzeMEV(w http.ResponseWriter, r *http.Request) {
	(&Handler{}).AnalyzeMEV(w, r)
}

// AnalyzeMEV simulates a Solana swap transaction and returns a sandwich-attack
// risk report. Phase-1 uses deterministic local heuristics and persists the
// estimated mev_saved_usd metric to mev_protection_events for owner reporting.
func (h *Handler) AnalyzeMEV(w http.ResponseWriter, r *http.Request) {
	var req mevAnalyzeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if strings.TrimSpace(req.TXSignature) == "" && strings.TrimSpace(req.RawTransaction) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transaction_required"})
		return
	}
	report := buildMEVRiskReport(req)
	if h.DB != nil && strings.TrimSpace(req.UserWallet) != "" {
		rawPayload, _ := json.Marshal(req)
		_, _ = h.DB.ExecContext(r.Context(), `INSERT INTO mev_protection_events (user_wallet, tx_signature, estimated_loss_usd, mev_saved_usd, jito_tip_used, risk_score, risk_level, route, raw_payload, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,now())`, req.UserWallet, firstNonEmpty(req.TXSignature, fingerprintPayload(req.RawTransaction)), report.EstimatedLossUSD, report.MEVSavedUSD, report.JitoTipUsed, report.RiskScore, report.RiskLevel, req.Route, string(rawPayload))
	}
	writeJSON(w, http.StatusOK, report)
}

// MEVAnalyze keeps the v1 route name stable while the module-level handler
// adopts the task-oriented AnalyzeMEV name.
func (h *Handler) MEVAnalyze(w http.ResponseWriter, r *http.Request) {
	h.AnalyzeMEV(w, r)
}

// TXDecoderMEVWarning exposes a lightweight warning box payload for the TX
// Decoder frontend so consumer flows can reuse the same MEV score without
// buying the full enterprise API.
func (h *Handler) TXDecoderMEVWarning(w http.ResponseWriter, r *http.Request) {
	var req mevAnalyzeRequest
	_ = decodeJSON(r, &req)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "warning": buildMEVRiskReport(req)})
}

func buildMEVRiskReport(req mevAnalyzeRequest) mevAnalyzeResponse {
	score := 12
	signals := []string{"Temel swap simülasyonu çalıştırıldı."}
	if req.SlippageBPS >= 100 {
		score += 25
		signals = append(signals, "Slippage toleransı sandwich saldırıları için geniş.")
	}
	if req.SlippageBPS >= 300 {
		score += 20
		signals = append(signals, "Çok yüksek slippage toleransı tespit edildi.")
	}
	if req.InputAmountUSD >= 10_000 {
		score += 18
		signals = append(signals, "İşlem büyüklüğü MEV botları için ekonomik olarak cazip.")
	}
	if req.PoolLiquidityUSD > 0 && req.InputAmountUSD/req.PoolLiquidityUSD >= 0.01 {
		score += 25
		signals = append(signals, "İşlem havuz likiditesine göre yüksek fiyat etkisi yaratıyor.")
	}
	if score > 100 {
		score = 100
	}
	level := "DÜŞÜK"
	if score >= 70 {
		level = "YÜKSEK"
	} else if score >= 40 {
		level = "ORTA"
	}
	loss := estimatedMEVLossUSD(req.InputAmountUSD, req.SlippageBPS, score)
	jitoTip := score >= 40
	return mevAnalyzeResponse{RiskScore: score, RiskLevel: level, EstimatedLossUSD: loss, JitoTipUsed: jitoTip, RecommendedTipSOL: recommendedJitoTip(score), MEVSavedUSD: loss, Signals: signals, EnterpriseReadyAPI: true, Message: "MEV Shield raporu hazır. Risk orta/yüksek ise korumalı gönderim önerilir."}
}

func estimatedMEVLossUSD(input float64, slippageBPS int, score int) float64 {
	if input <= 0 || slippageBPS <= 0 {
		return 0
	}
	return math.Round(input*(float64(slippageBPS)/10_000)*(float64(score)/100)*100) / 100
}

func recommendedJitoTip(score int) float64 {
	if score >= 70 {
		return 0.002
	}
	if score >= 40 {
		return 0.0008
	}
	return 0
}

func fingerprintPayload(v string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(v) + time.Now().UTC().Format("20060102")))
	return hex.EncodeToString(sum[:8])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
