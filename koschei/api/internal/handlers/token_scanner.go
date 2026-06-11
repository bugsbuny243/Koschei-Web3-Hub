package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type tokenScanRequest struct {
	Mint    string `json:"mint"`
	Address string `json:"address"`
	Network string `json:"network"`
}

type tokenScanResponse struct {
	Mint                 string   `json:"mint"`
	Network              string   `json:"network"`
	Score                int      `json:"score"`
	RiskLevel            string   `json:"risk_level"`
	Supply               string   `json:"supply"`
	Decimals             int      `json:"decimals"`
	MintAuthority        string   `json:"mint_authority,omitempty"`
	FreezeAuthority      string   `json:"freeze_authority,omitempty"`
	LargestHolderPercent float64  `json:"largest_holder_percent"`
	TopTenPercent        float64  `json:"top_ten_percent"`
	Findings             []string `json:"findings"`
	Disclaimer           string   `json:"disclaimer"`
}

type rpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (h *Handler) TokenScan(w http.ResponseWriter, r *http.Request) {
	var req tokenScanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	mint := strings.TrimSpace(req.Mint)
	if mint == "" {
		mint = strings.TrimSpace(req.Address)
	}
	if mint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mint required"})
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
	isPrivileged, credits, _ := h.userCreditsAndRole(claims.Sub)
	const toolCost = 1
	if !isPrivileged && credits < toolCost {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}

	result, err := h.tokenService().ScanToken(r.Context(), req.Network, mint)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "token mint not found or RPC request failed"})
		return
	}

	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "token_scan"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}
	mintAuthority, freezeAuthority := "", ""
	if result.Token.MintAuthority != nil {
		mintAuthority = *result.Token.MintAuthority
	}
	if result.Token.FreezeAuthority != nil {
		freezeAuthority = *result.Token.FreezeAuthority
	}
	writeJSON(w, http.StatusOK, tokenScanResponse{Mint: mint, Network: req.Network, Score: result.Score, RiskLevel: result.RiskLevel, Supply: result.Token.SupplyRaw, Decimals: result.Token.Decimals, MintAuthority: mintAuthority, FreezeAuthority: freezeAuthority, LargestHolderPercent: result.Token.LargestHolderPercent, TopTenPercent: result.Token.TopTenPercent, Findings: result.Findings, Disclaimer: result.Disclaimer})
}

func callSolanaRPC(client *http.Client, rpcURL, method string, params interface{}, target interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return err
	}
	resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rpc status %d", resp.StatusCode)
	}
	var envelope rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("rpc error: %s", envelope.Error.Message)
	}
	return json.Unmarshal(envelope.Result, target)
}

func roundPercent(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
