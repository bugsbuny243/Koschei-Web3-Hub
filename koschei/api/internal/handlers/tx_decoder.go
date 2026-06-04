package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

	// Build context for AI
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Solana Transaction Analysis\nSignature: %s\nNetwork: %s\nStatus: %s\nFee: %s\n",
		req.Signature, req.Network, txStatus, feeSol))

	if len(programNames) > 0 {
		sb.WriteString("Programs involved: " + strings.Join(programNames, ", ") + "\n")
	}

	if result.Transaction != nil && result.Transaction.Message != nil {
		sb.WriteString(fmt.Sprintf("Instruction count: %d\n", len(result.Transaction.Message.Instructions)))
		sb.WriteString(fmt.Sprintf("Account count: %d\n", len(result.Transaction.Message.AccountKeys)))

		// Balance changes
		if result.Meta != nil && len(result.Meta.PreBalances) > 0 {
			sb.WriteString("Balance changes:\n")
			for i, acc := range result.Transaction.Message.AccountKeys {
				if i < len(result.Meta.PreBalances) && i < len(result.Meta.PostBalances) {
					diff := result.Meta.PostBalances[i] - result.Meta.PreBalances[i]
					if diff != 0 {
						sb.WriteString(fmt.Sprintf("  %s: %+.6f SOL\n", acc.Pubkey[:min(8, len(acc.Pubkey))], float64(diff)/1e9))
					}
				}
			}
		}

		if result.Meta != nil && len(result.Meta.LogMessages) > 0 {
			sb.WriteString("Logs (first 3):\n")
			for i, log := range result.Meta.LogMessages {
				if i >= 3 {
					break
				}
				sb.WriteString("  " + log + "\n")
			}
		}
	}

	// Call Together AI
	aiKey := os.Getenv("TOGETHER_API_KEY")
	model := os.Getenv("TOGETHER_MODEL")
	if model == "" {
		model = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	}

	prompt := fmt.Sprintf(`You are a Solana blockchain security expert. Analyze this transaction and respond ONLY with a valid JSON object.

Transaction data:
%s

Respond with this exact JSON (no markdown, no extra text):
{
  "explanation": "1-2 sentence plain English explanation of what happened",
  "actions": ["list", "of", "key", "actions"],
  "risk_level": "SAFE or WARNING or DANGER",
  "risk_reason": "only if WARNING or DANGER, explain why"
}`, sb.String())

	aiBody, _ := json.Marshal(map[string]interface{}{
		"model":       model,
		"max_tokens":  400,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.2,
	})

	aiReq, _ := http.NewRequest("POST", "https://api.together.xyz/v1/chat/completions", bytes.NewReader(aiBody))
	aiReq.Header.Set("Authorization", "Bearer "+aiKey)
	aiReq.Header.Set("Content-Type", "application/json")

	aiResp, err := client.Do(aiReq)

	explanation := "Transaction fetched successfully. AI explanation unavailable."
	riskLevel := "UNKNOWN"
	riskReason := ""

	if err == nil && aiResp != nil {
		defer aiResp.Body.Close()
		aiData, _ := io.ReadAll(aiResp.Body)
		var aiResult struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(aiData, &aiResult) == nil && len(aiResult.Choices) > 0 {
			content := strings.TrimSpace(aiResult.Choices[0].Message.Content)
			content = strings.TrimPrefix(content, "```json")
			content = strings.TrimPrefix(content, "```")
			content = strings.TrimSuffix(content, "```")
			content = strings.TrimSpace(content)
			var aiParsed struct {
				Explanation string   `json:"explanation"`
				Actions     []string `json:"actions"`
				RiskLevel   string   `json:"risk_level"`
				RiskReason  string   `json:"risk_reason"`
			}
			if json.Unmarshal([]byte(content), &aiParsed) == nil {
				if aiParsed.Explanation != "" {
					explanation = aiParsed.Explanation
				}
				if aiParsed.RiskLevel != "" {
					riskLevel = aiParsed.RiskLevel
				}
				riskReason = aiParsed.RiskReason
			}
		}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
