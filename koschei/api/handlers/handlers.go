package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api"
)

// Health check
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "Koschei API",
	})
}

// WalletScore - real implementation with Alchemy
type WalletScoreRequest struct {
	Address string `json:"address"`
}

type WalletScoreResponse struct {
	Score     int    `json:"score"`
	RiskLevel string `json:"risk_level"`
	Message   string `json:"message"`
}
}

func WalletScore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req WalletScoreRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Address == "" {
		http.Error(w, "Invalid request: address required", http.StatusBadRequest)
		return
	}

	// Call Alchemy to get account info
	accountInfo, err := api.GetAccountInfo(req.Address)
	if err != nil {
		// Fallback response if Alchemy fails
		json.NewEncoder(w).Encode(WalletScoreResponse{
			Score:     50,
			RiskLevel: "medium",
			Message:   "Could not fetch wallet data. Please try again later.",
		})
		return
	}

	// Simple scoring logic (placeholder - can be improved)
	score := 60
	riskLevel := "medium"

	// If account exists and has data, give higher score
	if result, ok := accountInfo["result"].(map[string]interface{}); ok {
		if result != nil {
			score = 75
			riskLevel = "low"
		}
	}

	json.NewEncoder(w).Encode(WalletScoreResponse{
		Score:     score,
		RiskLevel: riskLevel,
		Message:   "Wallet score calculated successfully.",
	})
}

// TX Decode
func TxDecode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "TX Decoder endpoint ready. Real implementation coming soon.",
	})
}

// Token Scan
func TokenScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"risk_level": "medium",
		"message":    "Token Scanner endpoint ready. Real implementation coming soon.",
	})
}