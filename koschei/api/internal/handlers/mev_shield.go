package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
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
	ProtectedSend    bool    `json:"protected_send"`
}

type mevProtectedSendRequest struct {
	UserWallet     string   `json:"user_wallet"`
	TXSignature    string   `json:"tx_signature"`
	RawTransaction string   `json:"raw_transaction"`
	Transactions   []string `json:"transactions"`
	Route          string   `json:"route"`
	InputAmountUSD float64  `json:"input_amount_usd"`
	SlippageBPS    int      `json:"slippage_bps"`
}

type jitoRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
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
	JitoBundleID       string   `json:"jito_bundle_id,omitempty"`
	JitoStatus         string   `json:"jito_status,omitempty"`
	Message            string   `json:"message"`
}

// AnalyzeMEV provides a package-level compatibility endpoint for minimal integrations
// that do not instantiate Handler. Production routes should use Handler.AnalyzeMEV
// so database persistence is available.
func AnalyzeMEV(w http.ResponseWriter, r *http.Request) {
	(&Handler{}).AnalyzeMEV(w, r)
}

// AnalyzeMEV analyzes a submitted Solana swap transaction with deterministic heuristics and returns a sandwich-attack
// risk report. Phase-1 uses deterministic local heuristics and persists the
// estimated mev_saved_usd metric to mev_protection_events for owner reporting.
func (h *Handler) AnalyzeMEV(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if _, err := h.requirePremiumOutput(claims.Sub); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

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
	if req.ProtectedSend && strings.TrimSpace(req.RawTransaction) != "" {
		bundleID, status, err := h.submitJitoBundle(r.Context(), []string{req.RawTransaction})
		report.JitoBundleID = bundleID
		report.JitoStatus = status
		if err != nil {
			report.JitoStatus = "failed"
			report.Signals = append(report.Signals, "Jito Bundle API gönderimi başarısız: "+err.Error())
		} else {
			report.JitoTipUsed = true
			report.Signals = append(report.Signals, "Korumalı Gönder işlemi Jito Bundle API üzerinden yönlendirildi.")
		}
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()
	if strings.TrimSpace(req.UserWallet) != "" {
		rawPayload, _ := json.Marshal(req)
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO mev_protection_events (user_wallet, tx_signature, estimated_loss_usd, mev_saved_usd, jito_tip_used, risk_score, risk_level, route, raw_payload, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,now())`, req.UserWallet, firstNonEmpty(req.TXSignature, fingerprintPayload(req.RawTransaction)), report.EstimatedLossUSD, report.MEVSavedUSD, report.JitoTipUsed, report.RiskScore, report.RiskLevel, req.Route, string(rawPayload)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if err := h.applyCreditChargeTxWithReason(tx, claims.Sub, claims.Email, "mev_shield"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// ProtectedSendMEV forwards an already signed Solana transaction bundle to the
// Jito Bundle API. Koschei never requests private keys or signs on behalf of a
// user; the frontend must provide signed, serialized transactions.
func (h *Handler) ProtectedSend(w http.ResponseWriter, r *http.Request) {
	h.ProtectedSendMEV(w, r)
}

func (h *Handler) ProtectedSendMEV(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if _, err := h.requirePremiumOutput(claims.Sub); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

	var req mevProtectedSendRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	txs := make([]string, 0, len(req.Transactions)+1)
	if strings.TrimSpace(req.RawTransaction) != "" {
		txs = append(txs, strings.TrimSpace(req.RawTransaction))
	}
	for _, tx := range req.Transactions {
		if strings.TrimSpace(tx) != "" {
			txs = append(txs, strings.TrimSpace(tx))
		}
	}
	if len(txs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "signed_transaction_required"})
		return
	}

	bundleID, status, err := h.submitJitoBundle(r.Context(), txs)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "jito_bundle_failed", "message": err.Error(), "jito_status": status})
		return
	}
	report := buildMEVRiskReport(mevAnalyzeRequest{InputAmountUSD: req.InputAmountUSD, SlippageBPS: req.SlippageBPS, RawTransaction: txs[0], TXSignature: req.TXSignature, UserWallet: req.UserWallet, Route: req.Route})
	report.JitoTipUsed = true
	report.JitoBundleID = bundleID
	report.JitoStatus = status
	report.Message = "Korumalı Gönder başarılı: işlem Jito Bundle API üzerinden yönlendirildi."
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()
	if strings.TrimSpace(req.UserWallet) != "" {
		rawPayload, _ := json.Marshal(req)
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO mev_protection_events (user_wallet, tx_signature, estimated_loss_usd, mev_saved_usd, jito_tip_used, risk_score, risk_level, route, raw_payload, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,now())`, req.UserWallet, firstNonEmpty(req.TXSignature, fingerprintPayload(txs[0])), report.EstimatedLossUSD, report.MEVSavedUSD, true, report.RiskScore, report.RiskLevel, firstNonEmpty(req.Route, "jito_bundle"), string(rawPayload)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if err := h.applyCreditChargeTxWithReason(tx, claims.Sub, claims.Email, "mev_protected_send"); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "jito_bundle_id": bundleID, "jito_status": status, "report": report})
}

func (h *Handler) submitJitoBundle(ctx context.Context, transactions []string) (string, string, error) {
	url := strings.TrimSpace(os.Getenv("JITO_BUNDLE_URL"))
	if url == "" {
		url = "https://mainnet.block-engine.jito.wtf/api/v1/bundles"
	}
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "sendBundle", "params": []any{transactions}}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "request_build_failed", err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth := strings.TrimSpace(os.Getenv("JITO_AUTH_TOKEN")); auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", "network_error", err
	}
	defer res.Body.Close()
	var rpc jitoRPCResponse
	if err := json.NewDecoder(res.Body).Decode(&rpc); err != nil {
		return "", "invalid_response", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "http_error", errors.New(res.Status)
	}
	if rpc.Error != nil {
		return "", "rpc_error", errors.New(rpc.Error.Message)
	}
	bundleID := strings.Trim(string(rpc.Result), "\"")
	return bundleID, "submitted", nil
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
