package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"
)

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"neonAuthUrl": os.Getenv("EXPO_PUBLIC_NEON_AUTH_URL"),
		"version":     "2.0.0",
	})
}

func (h *Handler) Provision(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	claims, err := parseAndVerifyNeonJWT(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"ok":    "true",
		"sub":   claims.Sub,
		"email": claims.Email,
	})
}

type chainHealthResponse struct {
	OK       bool   `json:"ok"`
	Status   string `json:"status"`
	Chain    string `json:"chain"`
	Network  string `json:"network"`
	Provider string `json:"provider"`
}

func (h *Handler) Web3Health(w http.ResponseWriter, r *http.Request) {
	chain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("chain")))
	if chain == "" {
		chain = "solana"
	}
	type rpcConfig struct {
		url     string
		network string
		body    string
	}
	apiKey := os.Getenv("ALCHEMY_API_KEY")
	configs := map[string]rpcConfig{
		"solana":   {url: "https://solana-devnet.g.alchemy.com/v2/" + apiKey, network: "devnet", body: `{"jsonrpc":"2.0","id":1,"method":"getHealth"}`},
		"ethereum": {url: "https://eth-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"base":     {url: "https://base-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"arbitrum": {url: "https://arb-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"polygon":  {url: "https://polygon-amoy.g.alchemy.com/v2/" + apiKey, network: "amoy", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
		"optimism": {url: "https://opt-sepolia.g.alchemy.com/v2/" + apiKey, network: "sepolia", body: `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`},
	}
	cfg, ok := configs[chain]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown chain"})
		return
	}
	status := "online"
	if apiKey == "" {
		status = "no_api_key"
	} else {
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest(http.MethodPost, cfg.url, strings.NewReader(cfg.body))
		if err != nil {
			status = "error"
		} else {
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode >= http.StatusBadRequest {
				status = "error"
			} else {
				resp.Body.Close()
			}
		}
	}
	writeJSON(w, http.StatusOK, chainHealthResponse{OK: status == "online", Status: status, Chain: chain, Network: cfg.network, Provider: "Alchemy"})
}
