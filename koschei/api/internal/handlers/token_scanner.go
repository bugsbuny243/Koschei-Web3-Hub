package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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

	client := &http.Client{Timeout: 12 * time.Second}
	rpcURL := solanaRPCURL(req.Network, os.Getenv("ALCHEMY_API_KEY"))
	var supply struct {
		Value struct {
			Amount   string `json:"amount"`
			Decimals int    `json:"decimals"`
		} `json:"value"`
	}
	if err := h.callSolanaRPC(r.Context(), client, rpcURL, req.Network, "getTokenSupply", []interface{}{mint}, &supply); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "token mint not found or RPC request failed"})
		return
	}

	var account struct {
		Value *struct {
			Data struct {
				Parsed struct {
					Info struct {
						MintAuthority   *string `json:"mintAuthority"`
						FreezeAuthority *string `json:"freezeAuthority"`
					} `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"value"`
	}
	_ = h.callSolanaRPC(r.Context(), client, rpcURL, req.Network, "getAccountInfo", []interface{}{mint, map[string]string{"encoding": "jsonParsed"}}, &account)

	var largest struct {
		Value []struct {
			Amount string `json:"amount"`
		} `json:"value"`
	}
	_ = h.callSolanaRPC(r.Context(), client, rpcURL, req.Network, "getTokenLargestAccounts", []interface{}{mint}, &largest)

	total, _ := strconv.ParseFloat(supply.Value.Amount, 64)
	topOne, topTen := 0.0, 0.0
	for i, holder := range largest.Value {
		amount, _ := strconv.ParseFloat(holder.Amount, 64)
		if total > 0 && i < 10 {
			topTen += amount / total * 100
			if i == 0 {
				topOne = amount / total * 100
			}
		}
	}

	score := 100
	findings := []string{}
	mintAuthority, freezeAuthority := "", ""
	if account.Value != nil {
		if account.Value.Data.Parsed.Info.MintAuthority != nil {
			mintAuthority = *account.Value.Data.Parsed.Info.MintAuthority
			score -= 25
			findings = append(findings, "Mint authority is active and can create additional supply.")
		} else {
			findings = append(findings, "Mint authority is disabled.")
		}
		if account.Value.Data.Parsed.Info.FreezeAuthority != nil {
			freezeAuthority = *account.Value.Data.Parsed.Info.FreezeAuthority
			score -= 20
			findings = append(findings, "Freeze authority is active and can freeze token accounts.")
		} else {
			findings = append(findings, "Freeze authority is disabled.")
		}
	}
	if topOne >= 50 {
		score -= 35
		findings = append(findings, "The largest token account controls at least half of the supply.")
	} else if topOne >= 20 {
		score -= 20
		findings = append(findings, "The largest token account has a significant concentration.")
	}
	if topTen >= 80 {
		score -= 20
		findings = append(findings, "The ten largest token accounts control most of the supply.")
	}
	if score < 0 {
		score = 0
	}
	risk := "low"
	if score < 40 {
		risk = "high"
	} else if score < 70 {
		risk = "medium"
	}

	if !isPrivileged {
		if err := h.spendOutput(claims.Email, "token_scan"); err != nil {
			writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
			return
		}
	}

	disclaimer := "Koschei provides read-only risk signals based on public on-chain data. This is not financial advice."

	writeJSON(w, http.StatusOK, tokenScanResponse{
		Mint:                 mint,
		Network:              req.Network,
		Score:                score,
		RiskLevel:            risk,
		Supply:               supply.Value.Amount,
		Decimals:             supply.Value.Decimals,
		MintAuthority:        mintAuthority,
		FreezeAuthority:      freezeAuthority,
		LargestHolderPercent: roundPercent(topOne),
		TopTenPercent:        roundPercent(topTen),
		Findings:             findings,
		Disclaimer:           disclaimer,
	})
}

func (h *Handler) callSolanaRPC(ctx context.Context, client *http.Client, rpcURL, network, method string, params interface{}, target interface{}) error {
	if h != nil && h.SolanaRPC != nil {
		return h.SolanaRPC.Call(ctx, network, method, params, target, 0)
	}
	return callSolanaRPC(client, rpcURL, method, params, target)
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

