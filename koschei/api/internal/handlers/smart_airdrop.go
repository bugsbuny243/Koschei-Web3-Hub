package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"
)

type airdropCheckRequest struct {
	Address string `json:"address"`
	Network string `json:"network"`
}

type airdropSignal struct {
	Campaign string `json:"campaign"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

func (h *Handler) AirdropCheck(w http.ResponseWriter, r *http.Request) {
	var req airdropCheckRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Address) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address required"})
		return
	}
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	isPrivileged, outputs, err := h.userCreditsAndRole(claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if !isPrivileged && outputs <= 0 {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	client := &http.Client{Timeout: 12 * time.Second}
	rpcURL := solanaRPCURL(req.Network, os.Getenv("ALCHEMY_API_KEY"))
	var balance struct {
		Value int64 `json:"value"`
	}
	if err := callSolanaRPC(client, rpcURL, "getBalance", []interface{}{req.Address}, &balance); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "wallet lookup failed"})
		return
	}
	txCount := fetchRecentSignatureCount(client, rpcURL, req.Address)
	signals := []airdropSignal{
		{Campaign: "Koschei ecosystem", Status: eligibilityStatus(txCount >= 5), Reason: "Requires at least 5 recent on-chain transactions."},
		{Campaign: "Active Solana wallet", Status: eligibilityStatus(balance.Value > 0 && txCount > 0), Reason: "Requires a funded wallet with recent activity."},
	}
	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "airdrop_checker"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"address": req.Address, "network": req.Network, "balance_sol": float64(balance.Value) / 1e9, "recent_tx_count": txCount, "signals": signals, "checked_at": time.Now().UTC().Format(time.RFC3339), "disclaimer": "This checks public signals only and does not guarantee eligibility. Never connect a wallet or share a seed phrase to claim an airdrop."})
}

func eligibilityStatus(eligible bool) string {
	if eligible {
		return "potentially_eligible"
	}
	return "not_detected"
}
