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

func (h *Handler) RugRadarFeed(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, 503, map[string]string{"error": "db unavailable"})
		return
	}
	rows, err := h.DB.Query(`
	 SELECT mint_address, network, risk_score, risk_level,
	        risk_summary, is_renounced, is_frozen, tx_count,
	        submitted_by, created_at
	 FROM token_launches
	 ORDER BY created_at DESC
	 LIMIT 50
	`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	defer rows.Close()

	type launch struct {
		MintAddress string    `json:"mint_address"`
		Network     string    `json:"network"`
		RiskScore   int       `json:"risk_score"`
		RiskLevel   string    `json:"risk_level"`
		RiskSummary string    `json:"risk_summary"`
		IsRenounced bool      `json:"is_renounced"`
		IsFrozen    bool      `json:"is_frozen"`
		TxCount     int       `json:"tx_count"`
		SubmittedBy string    `json:"submitted_by"`
		CreatedAt   time.Time `json:"created_at"`
	}

	launches := []launch{}
	for rows.Next() {
		var l launch
		if err := rows.Scan(&l.MintAddress, &l.Network, &l.RiskScore,
			&l.RiskLevel, &l.RiskSummary, &l.IsRenounced, &l.IsFrozen,
			&l.TxCount, &l.SubmittedBy, &l.CreatedAt); err == nil {
			launches = append(launches, l)
		}
	}

	writeJSON(w, 200, map[string]interface{}{
		"launches":   launches,
		"total":      len(launches),
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) RugRadarSubmit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MintAddress string `json:"mint_address"`
		Network     string `json:"network"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MintAddress == "" {
		writeJSON(w, 400, map[string]string{"error": "mint_address required"})
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
	const toolCost = 2
	if !isPrivileged && credits < toolCost {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "insufficient_credits", "required": toolCost, "balance": credits})
		return
	}

	// Check if already scanned
	if h.DB != nil {
		var count int
		h.DB.QueryRow(`SELECT COUNT(*) FROM token_launches WHERE mint_address=$1`, req.MintAddress).Scan(&count)
		if count > 0 {
			writeJSON(w, 409, map[string]string{"error": "already_scanned", "message": "This token was already analyzed."})
			return
		}
	}

	// Run token scan
	apiKey := os.Getenv("ALCHEMY_API_KEY")
	rpcURL := solanaRPCURL(req.Network, apiKey)
	client := &http.Client{Timeout: 10 * time.Second}
	score := 50
	riskLevel := "UNKNOWN"
	riskSummary := "Analysis pending."
	isRenounced := false
	isFrozen := false
	txCount := 0

	// getAccountInfo
	acctBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getAccountInfo",
		"params": []interface{}{req.MintAddress, map[string]string{"encoding": "jsonParsed"}},
	})
	if resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(acctBody)); err == nil {
		defer resp.Body.Close()
		var result struct {
			Result struct {
				Value *struct {
					Data struct {
						Parsed *struct {
							Info *struct {
								MintAuthority   *string `json:"mintAuthority"`
								FreezeAuthority *string `json:"freezeAuthority"`
							} `json:"info"`
							Type string `json:"type"`
						} `json:"parsed"`
					} `json:"data"`
				} `json:"value"`
			} `json:"result"`
		}
		if d, _ := io.ReadAll(resp.Body); json.Unmarshal(d, &result) == nil && result.Result.Value != nil {
			if p := result.Result.Value.Data.Parsed; p != nil && p.Info != nil {
				isRenounced = p.Info.MintAuthority == nil
				isFrozen = p.Info.FreezeAuthority != nil
			}
		}
	}

	// Tx count
	sigBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 2,
		"method": "getSignaturesForAddress",
		"params": []interface{}{req.MintAddress, map[string]interface{}{"limit": 50}},
	})
	if resp, err := client.Post(rpcURL, "application/json", bytes.NewReader(sigBody)); err == nil {
		defer resp.Body.Close()
		var result struct {
			Result []struct{} `json:"result"`
		}
		if d, _ := io.ReadAll(resp.Body); json.Unmarshal(d, &result) == nil {
			txCount = len(result.Result)
		}
	}

	// Score
	score = 60
	if isRenounced {
		score += 20
	}
	if isFrozen {
		score -= 25
	}
	if txCount == 0 {
		score -= 20
	} else if txCount < 5 {
		score -= 10
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	switch {
	case score >= 80:
		riskLevel = "SAFE"
	case score >= 60:
		riskLevel = "LOW RISK"
	case score >= 40:
		riskLevel = "MEDIUM RISK"
	case score >= 20:
		riskLevel = "HIGH RISK"
	default:
		riskLevel = "DANGER"
	}

	// AI summary
	aiKey := os.Getenv("TOGETHER_API_KEY")
	if aiKey != "" {
		model := os.Getenv("TOGETHER_MODEL")
		if model == "" {
			model = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
		}
		prompt := fmt.Sprintf(`1 sentence Solana token security summary: score=%d, level=%s, mint_renounced=%v, freeze=%v, txs=%d. Be direct, no markdown.`,
			score, riskLevel, isRenounced, isFrozen, txCount)
		aiBody, _ := json.Marshal(map[string]interface{}{"model": model, "max_tokens": 80, "temperature": 0.3, "messages": []map[string]string{{"role": "user", "content": prompt}}})
		aiReq, _ := http.NewRequest("POST", "https://api.together.xyz/v1/chat/completions", bytes.NewReader(aiBody))
		aiReq.Header.Set("Authorization", "Bearer "+aiKey)
		aiReq.Header.Set("Content-Type", "application/json")
		if aiResp, err := client.Do(aiReq); err == nil {
			defer aiResp.Body.Close()
			var aiResult struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if d, _ := io.ReadAll(aiResp.Body); json.Unmarshal(d, &aiResult) == nil && len(aiResult.Choices) > 0 {
				if s := strings.TrimSpace(aiResult.Choices[0].Message.Content); s != "" {
					riskSummary = s
				}
			}
		}
	}

	// Save to DB
	submittedBy := "anonymous"
	if h.DB != nil {
		h.DB.Exec(`
		 INSERT INTO token_launches
		 (mint_address, network, risk_score, risk_level, risk_summary,
		  is_renounced, is_frozen, tx_count, submitted_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (mint_address) DO NOTHING`,
			req.MintAddress, req.Network, score, riskLevel, riskSummary,
			isRenounced, isFrozen, txCount, submittedBy)
	}

	if !isPrivileged && h.DB != nil {
		h.DB.Exec(`UPDATE app_user_profiles SET credits=credits-$1,updated_at=now() WHERE auth_subject=$2 AND credits>=$1`, toolCost, claims.Sub)
		h.DB.Exec(`INSERT INTO credit_events(email,amount,reason,created_at) VALUES($1,-$2,'rug_radar',now())`, claims.Email, toolCost)
	}

	writeJSON(w, 200, map[string]interface{}{
		"mint_address": req.MintAddress,
		"network":      req.Network,
		"risk_score":   score,
		"risk_level":   riskLevel,
		"risk_summary": riskSummary,
		"is_renounced": isRenounced,
		"is_frozen":    isFrozen,
		"tx_count":     txCount,
	})
}
