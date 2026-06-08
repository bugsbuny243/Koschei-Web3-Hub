package handlers

import (
	"encoding/json"
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

// Wallet Score - real implementation with Alchemy
func WalletScore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO: Parse address from request body
	// For now, return placeholder with note
	json.NewEncoder(w).Encode(map[string]interface{}{
		"score":      72,
		"risk_level": "medium",
		"message":    "Real Alchemy integration in progress. This is a placeholder response.",
		"note":      "Connect frontend to send wallet address in POST body.",
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