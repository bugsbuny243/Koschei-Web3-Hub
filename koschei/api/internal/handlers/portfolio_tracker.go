package handlers

import (
	"fmt"
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

type portfolioAsset struct {
	Symbol       string  `json:"symbol"`
	Mint         string  `json:"mint"`
	Amount       float64 `json:"amount"`
	AmountText   string  `json:"amount_text"`
	SharePercent float64 `json:"share_percent"`
}

type portfolioWallet struct {
	Address       string           `json:"address"`
	BalanceSOL    float64          `json:"balance_sol"`
	TokenCount    int              `json:"token_count"`
	RecentTxCount int              `json:"recent_tx_count"`
	Assets        []portfolioAsset `json:"assets"`
}

type portfolioDistribution struct {
	Label        string  `json:"label"`
	Value        float64 `json:"value"`
	SharePercent float64 `json:"share_percent"`
}

type portfolioTrackResponse struct {
	Network         string                  `json:"network"`
	Wallets         []portfolioWallet       `json:"wallets"`
	TotalBalanceSOL float64                 `json:"total_balance_sol"`
	AssetCount      int                     `json:"asset_count"`
	Assets          []portfolioAsset        `json:"assets"`
	Distribution    []portfolioDistribution `json:"asset_distribution"`
	Summary         string                  `json:"summary"`
	TrackedAt       string                  `json:"tracked_at"`
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
	toolCost := ToolCreditCost("portfolio_tracker")
	if !isPrivileged && outputs < toolCost {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse(toolCost, outputs))
		return
	}

	client := &http.Client{Timeout: 12 * time.Second}
	rpcURL := solanaRPCURL(req.Network, os.Getenv("ALCHEMY_API_KEY"))
	result := portfolioTrackResponse{Network: req.Network, Wallets: []portfolioWallet{}, Assets: []portfolioAsset{}, Distribution: []portfolioDistribution{}, TrackedAt: time.Now().UTC().Format(time.RFC3339)}
	seen := map[string]bool{}
	assetTotals := map[string]portfolioAsset{}
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
		assets := tokenAccountsToAssets(tokens.Value)
		wallet := portfolioWallet{Address: address, BalanceSOL: float64(balance.Value) / 1e9, TokenCount: len(tokens.Value), RecentTxCount: fetchRecentSignatureCount(client, rpcURL, address), Assets: assets}
		result.TotalBalanceSOL += wallet.BalanceSOL
		result.Wallets = append(result.Wallets, wallet)
		for _, asset := range assets {
			current := assetTotals[asset.Mint]
			current.Symbol = asset.Symbol
			current.Mint = asset.Mint
			current.Amount += asset.Amount
			current.AmountText = fmt.Sprintf("%.6f", current.Amount)
			assetTotals[asset.Mint] = current
		}
	}
	result.TotalBalanceSOL = roundPercent(result.TotalBalanceSOL)
	result.Assets = append(result.Assets, portfolioAsset{Symbol: "SOL", Mint: "native", Amount: result.TotalBalanceSOL, AmountText: fmt.Sprintf("%.4f SOL", result.TotalBalanceSOL)})
	for _, asset := range assetTotals {
		result.Assets = append(result.Assets, asset)
	}
	result.AssetCount = len(result.Assets)
	assignPortfolioShares(&result)
	result.Summary = fmt.Sprintf("Tracked %d wallet(s), %.4f total SOL and %d unique SOL/token assets. Token values are quantity-only unless pricing data is connected.", len(result.Wallets), result.TotalBalanceSOL, result.AssetCount)
	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "portfolio_tracker"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type jsonTokenAccount struct {
	Pubkey  string `json:"pubkey"`
	Account struct {
		Data struct {
			Parsed struct {
				Info struct {
					Mint        string `json:"mint"`
					TokenAmount struct {
						Amount         string   `json:"amount"`
						UIAmount       *float64 `json:"uiAmount"`
						UIAmountString string   `json:"uiAmountString"`
					} `json:"tokenAmount"`
				} `json:"info"`
			} `json:"parsed"`
		} `json:"data"`
	} `json:"account"`
}

func tokenAccountsToAssets(accounts []jsonTokenAccount) []portfolioAsset {
	assets := []portfolioAsset{}
	for _, account := range accounts {
		info := account.Account.Data.Parsed.Info
		mint := strings.TrimSpace(info.Mint)
		if mint == "" {
			continue
		}
		amount := 0.0
		if info.TokenAmount.UIAmount != nil {
			amount = *info.TokenAmount.UIAmount
		}
		if amount == 0 && strings.TrimSpace(info.TokenAmount.UIAmountString) == "0" {
			continue
		}
		amountText := info.TokenAmount.UIAmountString
		if amountText == "" {
			amountText = info.TokenAmount.Amount
		}
		assets = append(assets, portfolioAsset{Symbol: shortAssetSymbol(mint), Mint: mint, Amount: amount, AmountText: amountText})
	}
	return assets
}

func assignPortfolioShares(result *portfolioTrackResponse) {
	pseudoTotal := result.TotalBalanceSOL
	for _, asset := range result.Assets {
		if asset.Mint != "native" {
			pseudoTotal += 1
		}
	}
	if pseudoTotal <= 0 {
		return
	}
	for i := range result.Assets {
		value := result.Assets[i].Amount
		if result.Assets[i].Mint != "native" {
			value = 1
		}
		result.Assets[i].SharePercent = roundPercent(value / pseudoTotal * 100)
		result.Distribution = append(result.Distribution, portfolioDistribution{Label: result.Assets[i].Symbol, Value: value, SharePercent: result.Assets[i].SharePercent})
	}
}

func shortAssetSymbol(mint string) string {
	if len(mint) <= 10 {
		return mint
	}
	return mint[:4] + "…" + mint[len(mint)-4:]
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
	toolCost := ToolCreditCost("smart_money")
	if !isPrivileged && outputs < toolCost {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse(toolCost, outputs))
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
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse(ToolCreditCost("smart_money"), 0))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"network": "solana-mainnet", "accounts": accounts, "disclaimer": "Labels are informational watchlist references. Verify identities independently before acting."})
}
