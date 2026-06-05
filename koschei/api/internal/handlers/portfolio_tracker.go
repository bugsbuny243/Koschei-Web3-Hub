package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"
)

type portfolioTrackRequest struct {
	Address   string   `json:"address"`
	Addresses []string `json:"addresses"`
	Network   string   `json:"network"`
}

type portfolioWallet struct {
	Address       string  `json:"address"`
	BalanceSOL    float64 `json:"balance_sol"`
	TokenCount    int     `json:"token_count"`
	RecentTxCount int     `json:"recent_tx_count"`
}

type portfolioTrackResponse struct {
	Network         string            `json:"network"`
	Wallets         []portfolioWallet `json:"wallets"`
	TotalBalanceSOL float64           `json:"total_balance_sol"`
	TrackedAt       string            `json:"tracked_at"`
}

func (h *Handler) PortfolioTrack(w http.ResponseWriter, r *http.Request) {
	var req portfolioTrackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if strings.TrimSpace(req.Address) != "" {
		req.Addresses = append(req.Addresses, req.Address)
	}
	if len(req.Addresses) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one address required"})
		return
	}
	if len(req.Addresses) > 10 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a maximum of 10 addresses can be tracked"})
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
	result := portfolioTrackResponse{Network: req.Network, Wallets: []portfolioWallet{}, TrackedAt: time.Now().UTC().Format(time.RFC3339)}
	seen := map[string]bool{}
	for _, raw := range req.Addresses {
		address := strings.TrimSpace(raw)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		var balance struct {
			Value int64 `json:"value"`
		}
		if err := callSolanaRPC(client, rpcURL, "getBalance", []interface{}{address}, &balance); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "wallet lookup failed for " + address})
			return
		}
		var tokens struct {
			Value []jsonTokenAccount `json:"value"`
		}
		_ = callSolanaRPC(client, rpcURL, "getTokenAccountsByOwner", []interface{}{address, map[string]string{"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"}, map[string]string{"encoding": "jsonParsed"}}, &tokens)
		wallet := portfolioWallet{Address: address, BalanceSOL: float64(balance.Value) / 1e9, TokenCount: len(tokens.Value), RecentTxCount: fetchRecentSignatureCount(client, rpcURL, address)}
		result.TotalBalanceSOL += wallet.BalanceSOL
		result.Wallets = append(result.Wallets, wallet)
	}
	result.TotalBalanceSOL = roundPercent(result.TotalBalanceSOL)
	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "portfolio_tracker"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type jsonTokenAccount struct {
	Pubkey string `json:"pubkey"`
}

type smartMoneyAccount struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Address     string `json:"address"`
	Description string `json:"description"`
	ExplorerURL string `json:"explorer_url"`
}

func (h *Handler) SmartMoney(w http.ResponseWriter, r *http.Request) {
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
	accounts := []smartMoneyAccount{
		{Name: "Jupiter Aggregator v6", Category: "protocol", Address: "JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4", Description: "Public protocol account that can provide context for high-volume swap activity."},
		{Name: "Solana Vote Program", Category: "ecosystem", Address: "Vote111111111111111111111111111111111111111", Description: "Public ecosystem reference account for validator-related activity."},
	}
	for i := range accounts {
		accounts[i].ExplorerURL = "https://solscan.io/account/" + accounts[i].Address
	}
	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "smart_money"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"network": "solana-mainnet", "accounts": accounts, "disclaimer": "Labels are informational watchlist references. Verify identities independently before acting."})
}
