package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	shopierURLs := map[string]string{
		"starter":      configuredURL("SHOPIER_STARTER_URL", "https://www.shopier.com/TradeVisual/46531862"),
		"builder":      configuredURL("SHOPIER_BUILDER_URL", "https://www.shopier.com/TradeVisual/46531900"),
		"studio":       configuredURL("SHOPIER_STUDIO_URL", "https://www.shopier.com/TradeVisual/46531961"),
		"professional": configuredURL("SHOPIER_BUILDER_URL", "https://www.shopier.com/TradeVisual/46531900"),
		"enterprise":   configuredURL("SHOPIER_STUDIO_URL", "https://www.shopier.com/TradeVisual/46531961"),
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     "2.0.0",
		"neonAuthUrl": configuredPublicNeonAuthURL(),
		"payments": map[string]any{
			"provider":    "shopier",
			"mode":        "manual_owner_approval",
			"shopierUrls": shopierURLs,
		},
	})
}

func configuredURL(envKey, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		return value
	}
	return fallback
}

func (h *Handler) Provision(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	claims, err := parseAndVerifyNeonJWT(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !h.RequireDB(w) {
		return
	}
	summary, err := h.provisionMember(r.Context(), claims)
	if err != nil {
		log.Printf("provisionMember failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "account provisioning unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"ok":    "true",
		"sub":   claims.Sub,
		"email": summary.Email,
		"plan":  freePlanID,
	})
}

type chainHealthResponse struct {
	OK       bool   `json:"ok"`
	Status   string `json:"status"`
	Chain    string `json:"chain"`
	Network  string `json:"network"`
	Provider string `json:"provider"`
	Result   string `json:"result"`
	Error    string `json:"error"`
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
	errorText := ""
	if apiKey == "" {
		status = "no_api_key"
		errorText = "Alchemy API key is not configured"
	} else {
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest(http.MethodPost, cfg.url, strings.NewReader(cfg.body))
		if err != nil {
			status = "error"
			errorText = err.Error()
		} else {
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				status = "error"
				errorText = err.Error()
			} else {
				resp.Body.Close()
				if resp.StatusCode >= http.StatusBadRequest {
					status = "error"
					errorText = fmt.Sprintf("Alchemy returned HTTP %d", resp.StatusCode)
				}
			}
		}
	}
	result := status
	response := chainHealthResponse{OK: status == "online", Status: status, Chain: chain, Network: cfg.network, Provider: "Alchemy", Result: result, Error: errorText}
	h.recordChainHealth(chainHealthLog{Chain: chain, Network: cfg.network, Provider: "alchemy", Healthy: response.OK, Result: result, Error: errorText, CheckedAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, response)
}
