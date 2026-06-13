package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type txDecodeRequest struct {
	Signature string `json:"signature"`
	Network   string `json:"network"`
}

type txDecodeResponse struct {
	Signature   string      `json:"signature"`
	Network     string      `json:"network"`
	Status      string      `json:"status"`
	Explanation string      `json:"explanation"`
	RiskLevel   string      `json:"risk_level"`
	RiskReason  string      `json:"risk_reason,omitempty"`
	Programs    []string    `json:"programs"`
	FeeSol      string      `json:"fee_sol"`
	Raw         interface{} `json:"raw,omitempty"`
}

var knownPrograms = map[string]string{
	"11111111111111111111111111111111":             "System Program",
	"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA":  "SPL Token Program",
	"JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4":  "Jupiter Aggregator",
	"9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin": "Serum DEX",
	"metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s":  "Metaplex Token Metadata",
	"So11111111111111111111111111111111111111112":  "Wrapped SOL",
}

func (h *Handler) TXDecode(w http.ResponseWriter, r *http.Request) {
	var req txDecodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Signature == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "signature required"})
		return
	}
	if req.Network == "" {
		req.Network = "solana-devnet"
	}

	apiKey := os.Getenv("ALCHEMY_API_KEY")
	rpcURL := solanaRPCURL(req.Network, apiKey)

	// Fetch transaction from Solana RPC
	rpcBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTransaction",
		"params": []interface{}{
			req.Signature,
			map[string]interface{}{
				"encoding":                       "jsonParsed",
				"maxSupportedTransactionVersion": 0,
			},
		},
	})

	client := &http.Client{Timeout: 10 * time.Second}
	rpcResp, err := client.Post(rpcURL, "application/json", bytes.NewReader(rpcBody))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "RPC request failed"})
		return
	}
	defer rpcResp.Body.Close()
	rpcData, _ := io.ReadAll(rpcResp.Body)

	var rpcResult struct {
		Result *struct {
			Meta *struct {
				Err          interface{} `json:"err"`
				Fee          int64       `json:"fee"`
				PreBalances  []int64     `json:"preBalances"`
				PostBalances []int64     `json:"postBalances"`
				LogMessages  []string    `json:"logMessages"`
			} `json:"meta"`
			Transaction *struct {
				Message *struct {
					AccountKeys []struct {
						Pubkey   string `json:"pubkey"`
						Signer   bool   `json:"signer"`
						Writable bool   `json:"writable"`
					} `json:"accountKeys"`
					Instructions []struct {
						Program   string      `json:"program"`
						ProgramId string      `json:"programId"`
						Parsed    interface{} `json:"parsed"`
					} `json:"instructions"`
				} `json:"message"`
			} `json:"transaction"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(rpcData, &rpcResult); err != nil || rpcResult.Result == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found or invalid signature"})
		return
	}

	result := rpcResult.Result
	txStatus := "success"
	if result.Meta != nil && result.Meta.Err != nil {
		txStatus = "failed"
	}

	// Extract programs
	programSet := map[string]bool{}
	programNames := []string{}
	if result.Transaction != nil && result.Transaction.Message != nil {
		for _, ix := range result.Transaction.Message.Instructions {
			pid := ix.ProgramId
			if pid == "" {
				pid = ix.Program
			}
			if pid != "" && !programSet[pid] {
				programSet[pid] = true
				if name, ok := knownPrograms[pid]; ok {
					programNames = append(programNames, name)
				} else {
					short := pid
					if len(short) > 12 {
						short = short[:8] + "..." + short[len(short)-4:]
					}
					programNames = append(programNames, short)
				}
			}
		}
	}

	// Calculate fee
	feeSol := "unknown"
	if result.Meta != nil {
		feeSol = fmt.Sprintf("%.6f SOL", float64(result.Meta.Fee)/1e9)
	}

	explanation := fmt.Sprintf("Transaction fetched from Solana RPC with status %s, fee %s, and %d involved programs.", txStatus, feeSol, len(programNames))
	riskLevel := "SAFE"
	riskReason := ""
	if txStatus == "failed" {
		riskLevel = "WARNING"
		riskReason = "The transaction execution reported an on-chain error."
	} else if len(programNames) >= 6 {
		riskLevel = "WARNING"
		riskReason = "The transaction touches many programs; inspect routed or composite execution carefully."
	}

	writeJSON(w, http.StatusOK, txDecodeResponse{
		Signature:   req.Signature,
		Network:     req.Network,
		Status:      txStatus,
		Explanation: explanation,
		RiskLevel:   riskLevel,
		RiskReason:  riskReason,
		Programs:    programNames,
		FeeSol:      feeSol,
	})
}
