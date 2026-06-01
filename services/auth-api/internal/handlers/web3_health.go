package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"
)

type chainHealthResponse struct {
	OK       bool   `json:"ok"`
	Status   string `json:"status"`
	Chain    string `json:"chain"`
	Network  string `json:"network"`
	Provider string `json:"provider"`
}

func (h *Handler) Web3ChainHealth(w http.ResponseWriter, r *http.Request) {
	chain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("chain")))
	if chain == "" {
		chain = "solana"
	}

	apiKey := os.Getenv("ALCHEMY_API_KEY")

	type rpcConfig struct {
		url     string
		network string
		method  string
		body    string
	}

	configs := map[string]rpcConfig{
		"solana":   {url: "https://solana-devnet.g.alchemy.com/v2/" + apiKey, network: "devnet", method: "POST", body: `{"jsonrpc":"2.0","id":1,"method":"getHealth"}`},
		"ethereum": {url: "https://eth-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", method: "POST", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"base":     {url: "https://base-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", method: "POST", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"arbitrum": {url: "https://arb-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", method: "POST", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"polygon":  {url: "https://polygon-amoy.g.alchemy.com/v2/" + apiKey, network: "amoy", method: "POST", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"optimism": {url: "https://opt-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", method: "POST", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
	}

	cfg, ok := configs[chain]
	if !ok {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown chain"})
		return
	}

	status := "online"
	if apiKey == "" {
		status = "no_api_key"
	} else {
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest("POST", cfg.url, strings.NewReader(cfg.body))
		if err != nil || req == nil {
			status = "error"
		} else {
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode >= 400 {
				status = "error"
			} else {
				resp.Body.Close()
			}
		}
	}

	WriteJSON(w, http.StatusOK, chainHealthResponse{
		OK:       status == "online",
		Status:   status,
		Chain:    chain,
		Network:  cfg.network,
		Provider: "Alchemy",
	})
}
