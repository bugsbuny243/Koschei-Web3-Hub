package handlers

import (
	"encoding/json"
	"net/http"
)

// Health check
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"service": "Koschei API",
	})
}

// Wallet Score (placeholder - will connect to Alchemy later)
func WalletScore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"score":        78,
		"risk_level":   "medium",
		"message":      "This is a placeholder. Real implementation coming soon.",
	})
}

// TX Decode (placeholder)
func TxDecode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "TX Decoder endpoint ready. Real implementation coming soon.",
	})
}

// Token Scan (placeholder)
func TokenScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"risk_level": "medium",
		"message":    "Token Scanner endpoint ready. Real implementation coming soon.",
	})
}