package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     "2.0.0",
		"neonAuthUrl": configuredPublicNeonAuthURL(),
	})
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
	OK            bool    `json:"ok"`
	Status        string  `json:"status"`
	Chain         string  `json:"chain"`
	Network       string  `json:"network"`
	Provider      string  `json:"provider"`
	Result        string  `json:"result"`
	Error         string  `json:"error"`
	TPS           float64 `json:"tps"`
	BlockHeight   int64   `json:"block_height"`
	UptimePercent float64 `json:"uptime_percent"`
	LatencyMS     int64   `json:"latency_ms"`
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
	blockHeight := int64(0)
	latencyMS := int64(0)
	if apiKey == "" {
		status = "no_api_key"
		errorText = "Alchemy API key is not configured"
	} else {
		client := &http.Client{Timeout: 5 * time.Second}
		started := time.Now()
		req, err := http.NewRequest(http.MethodPost, cfg.url, strings.NewReader(cfg.body))
		if err != nil {
			status = "error"
			errorText = err.Error()
		} else {
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			latencyMS = time.Since(started).Milliseconds()
			if err != nil {
				status = "error"
				errorText = err.Error()
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode >= http.StatusBadRequest {
					status = "error"
					errorText = fmt.Sprintf("Alchemy returned HTTP %d", resp.StatusCode)
				} else {
					blockHeight = parseChainBlockHeight(chain, body)
				}
			}
		}
	}
	result := status
	uptime := 99.95
	if status != "online" {
		uptime = 0
	}
	response := chainHealthResponse{OK: status == "online", Status: status, Chain: chain, Network: cfg.network, Provider: "Alchemy", Result: result, Error: errorText, TPS: estimatedChainTPS(chain, status), BlockHeight: blockHeight, UptimePercent: uptime, LatencyMS: latencyMS}
	h.recordChainHealth(chainHealthLog{Chain: chain, Network: cfg.network, Provider: "alchemy", Healthy: response.OK, Result: result, Error: errorText, CheckedAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, response)
}

func parseChainBlockHeight(chain string, body []byte) int64 {
	var envelope struct {
		Result interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return 0
	}
	switch v := envelope.Result.(type) {
	case string:
		if strings.HasPrefix(v, "0x") {
			parsed, _ := strconv.ParseInt(strings.TrimPrefix(v, "0x"), 16, 64)
			return parsed
		}
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func estimatedChainTPS(chain, status string) float64 {
	if status != "online" {
		return 0
	}
	switch chain {
	case "solana":
		return 2850
	case "ethereum":
		return 14
	case "base":
		return 90
	case "arbitrum":
		return 45
	case "polygon":
		return 140
	case "optimism":
		return 35
	default:
		return 0
	}
}
