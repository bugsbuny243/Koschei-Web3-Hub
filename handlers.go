package main

import (
	"encoding/json"
	"net/http"
)

type AnalyzeRequest struct {
	Input string `json:"input"`
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Geçersiz veri", http.StatusBadRequest)
		return
	}

	// Burada "Input" tipini belirle:
	// 1. Solana Adresi mi? (44 karakter civarı)
	// 2. Transaction Hash mi? (64-88 karakter)
	// 3. Token Adresi mi?
	
	// Örnek logic:
	result := map[string]string{
		"status": "success",
		"type":   "detected_input", 
		"data":   "Burada API'den (Alchemy/Helius) gelen gerçek veri dönecek.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
