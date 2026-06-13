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

type walletScoreRequest struct {
	Address string `json:"address"`
	Network string `json:"network"`
}

type walletScoreResponse struct {
	Address    string   `json:"address"`
	Network    string   `json:"network"`
	Score      int      `json:"score"`
	Level      string   `json:"level"`
	LevelColor string   `json:"level_color"`
	Summary    string   `json:"summary"`
	Badges     []string `json:"badges"`
	TxCount    int      `json:"tx_count"`
	BalanceSol string   `json:"balance_sol"`
	ActiveDays int      `json:"active_days"`
	FirstSeen  string   `json:"first_seen"`
	LastSeen   string   `json:"last_seen"`
}

func isValidSolanaAddress(addr string) bool {
	decoded, ok := base58Decode(addr)
	return ok && len(decoded) == 32
}

func (h *Handler) WalletScore(w http.ResponseWriter, r *http.Request) {
	var req walletScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address required"})
		return
	}
	if !isValidSolanaAddress(req.Address) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid solana address"})
		return
	}
	if req.Network == "" {
		req.Network = "solana-devnet"
	}

	apiKey := os.Getenv("ALCHEMY_API_KEY")
	rpcURL := solanaRPCURL(req.Network, apiKey)
	client := &http.Client{Timeout: 10 * time.Second}

	// ── 1. getAccountInfo ──
	acctBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "getAccountInfo",
		"params": []interface{}{req.Address, map[string]string{"encoding": "jsonParsed"}},
	})
	acctResp, err := client.Post(rpcURL, "application/json", bytes.NewReader(acctBody))
	balanceSol := "0 SOL"
	balanceLamports := int64(0)
	accountExists := false
	if err == nil {
		defer acctResp.Body.Close()
		var acctResult struct {
			Result struct {
				Value *struct {
					Lamports int64 `json:"lamports"`
				} `json:"value"`
			} `json:"result"`
		}
		if d, _ := io.ReadAll(acctResp.Body); json.Unmarshal(d, &acctResult) == nil && acctResult.Result.Value != nil {
			balanceLamports = acctResult.Result.Value.Lamports
			balanceSol = fmt.Sprintf("%.4f SOL", float64(balanceLamports)/1e9)
			accountExists = true
		}
	}

	if !accountExists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "wallet not found on this network"})
		return
	}

	// ── 2. getSignaturesForAddress ──
	sigBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 2,
		"method": "getSignaturesForAddress",
		"params": []interface{}{req.Address, map[string]interface{}{"limit": 100}},
	})
	sigResp, err := client.Post(rpcURL, "application/json", bytes.NewReader(sigBody))
	txCount := 0
	failedCount := 0
	firstSeen := ""
	lastSeen := ""
	activeDays := 0
	var latestBlockTime int64

	if err == nil {
		defer sigResp.Body.Close()
		var sigResult struct {
			Result []struct {
				Err       interface{} `json:"err"`
				BlockTime *int64      `json:"blockTime"`
			} `json:"result"`
		}
		if d, _ := io.ReadAll(sigResp.Body); json.Unmarshal(d, &sigResult) == nil {
			txCount = len(sigResult.Result)
			var earliest, latest int64
			for _, s := range sigResult.Result {
				if s.Err != nil {
					failedCount++
				}
				if s.BlockTime != nil {
					if earliest == 0 || *s.BlockTime < earliest {
						earliest = *s.BlockTime
					}
					if *s.BlockTime > latest {
						latest = *s.BlockTime
						latestBlockTime = *s.BlockTime
					}
				}
			}
			if earliest > 0 {
				firstSeen = time.Unix(earliest, 0).Format("Jan 2006")
				lastSeen = time.Unix(latest, 0).Format("Jan 2, 2006")
				activeDays = int(time.Since(time.Unix(earliest, 0)).Hours() / 24)
			}
		}
	}

	// ── 3. Score calculation ──
	score := 0

	// Balance points (max 15)
	if balanceLamports >= 100*1e9 {
		score += 15
	} else if balanceLamports >= 1*1e9 {
		score += 10
	} else if balanceLamports >= 1e8 {
		score += 5
	}

	// Transaction count (max 20)
	if txCount >= 500 {
		score += 20
	} else if txCount >= 100 {
		score += 15
	} else if txCount >= 20 {
		score += 10
	} else if txCount > 0 {
		score += 5
	}

	// Account age (max 25)
	if activeDays >= 365 {
		score += 25
	} else if activeDays >= 180 {
		score += 18
	} else if activeDays >= 90 {
		score += 12
	} else if activeDays >= 30 {
		score += 6
	}

	// Recent activity (max 20)
	if lastSeen != "" && activeDays > 0 && latestBlockTime > 0 {
		daysSinceLast := int(time.Since(time.Unix(latestBlockTime, 0)).Hours() / 24)
		if daysSinceLast < 7 {
			score += 20
		} else if daysSinceLast < 30 {
			score += 12
		} else if daysSinceLast < 90 {
			score += 6
		}
	}

	// Low failure rate (max 20)
	if txCount > 0 {
		failRate := float64(failedCount) / float64(txCount)
		if failRate < 0.05 {
			score += 20
		} else if failRate < 0.15 {
			score += 12
		} else if failRate < 0.3 {
			score += 5
		}
	} else {
		score += 10
	}

	if score > 100 {
		score = 100
	}

	// Level
	level, levelColor := "UNKNOWN", "#888"
	switch {
	case score >= 80:
		level, levelColor = "TRUSTED", "#00ff88"
	case score >= 60:
		level, levelColor = "GOOD", "#00ccff"
	case score >= 40:
		level, levelColor = "NEUTRAL", "#ffaa00"
	case score >= 20:
		level, levelColor = "RISKY", "#ff7700"
	default:
		level, levelColor = "DANGER", "#ff4466"
	}

	// Badges
	badges := []string{}
	if balanceLamports >= 100*1e9 {
		badges = append(badges, "🐋 Whale")
	}
	if txCount >= 500 {
		badges = append(badges, "⚡ Power User")
	} else if txCount >= 100 {
		badges = append(badges, "🔄 Active Trader")
	}
	if activeDays >= 365 {
		badges = append(badges, "💎 OG Holder")
	}
	if activeDays < 30 && txCount > 0 {
		badges = append(badges, "🆕 New Account")
	}
	if failedCount == 0 && txCount > 5 {
		badges = append(badges, "✅ Clean History")
	}
	if len(badges) == 0 {
		badges = append(badges, "👤 Standard Account")
	}

	// ── 4. Real-data summary (AI summaries are centralized in the unified router) ──
	summary := fmt.Sprintf("Wallet with %d fetched transactions, balance of %s, active for %d days. Score: %d/100 (%s).", txCount, balanceSol, activeDays, score, level)

	writeJSON(w, http.StatusOK, walletScoreResponse{
		Address:    req.Address,
		Network:    req.Network,
		Score:      score,
		Level:      level,
		LevelColor: levelColor,
		Summary:    summary,
		Badges:     badges,
		TxCount:    txCount,
		BalanceSol: balanceSol,
		ActiveDays: activeDays,
		FirstSeen:  firstSeen,
		LastSeen:   lastSeen,
	})
}

func solanaRPCURL(network string, apiKey string) string {
	if apiKey != "" {
		switch strings.ToLower(network) {
		case "solana-mainnet", "mainnet", "mainnet-beta", "solana-mainnet-beta":
			return "https://solana-mainnet.g.alchemy.com/v2/" + apiKey
		case "solana-devnet", "devnet":
			return "https://solana-devnet.g.alchemy.com/v2/" + apiKey
		case "solana-testnet", "testnet":
			return "https://solana-testnet.g.alchemy.com/v2/" + apiKey
		}
	}

	switch strings.ToLower(network) {
	case "solana-mainnet", "mainnet", "mainnet-beta", "solana-mainnet-beta":
		return "https://api.mainnet-beta.solana.com"
	case "solana-testnet", "testnet":
		return "https://api.testnet.solana.com"
	default:
		return "https://api.devnet.solana.com"
	}
}
