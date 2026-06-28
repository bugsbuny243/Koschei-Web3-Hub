package web3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/cache"
)

type SolanaRPC struct {
	Client        *http.Client
	Cache         cache.Cache
	KeyPrefix     string
	AlchemyAPIKey string
}

type solanaRPCEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewSolanaRPC(c cache.Cache) *SolanaRPC {
	if c == nil {
		c = cache.NewNoop()
	}
	prefix := strings.TrimSpace(os.Getenv("CACHE_KEY_PREFIX"))
	if prefix == "" {
		prefix = "koschei"
	}
	return &SolanaRPC{Client: &http.Client{Timeout: 12 * time.Second}, Cache: c, KeyPrefix: prefix, AlchemyAPIKey: os.Getenv("ALCHEMY_API_KEY")}
}

func (s *SolanaRPC) URL(network string) string {
	return configuredSolanaRPCURL(network, s.AlchemyAPIKey)
}

func configuredSolanaRPCURL(network, apiKey string) string {
	if isSolanaMainnet(network) {
		for _, key := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
			if value := strings.TrimSpace(os.Getenv(key)); value != "" {
				return value
			}
		}
		if strings.TrimSpace(apiKey) != "" {
			return "https://solana-mainnet.g.alchemy.com/v2/" + strings.TrimSpace(apiKey)
		}
		return "https://api.mainnet-beta.solana.com"
	}
	for _, key := range []string{"SOLANA_DEVNET_RPC_URL", "ALCHEMY_SOLANA_DEVNET_RPC_URL"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if strings.TrimSpace(apiKey) != "" {
		return "https://solana-devnet.g.alchemy.com/v2/" + strings.TrimSpace(apiKey)
	}
	return "https://api.devnet.solana.com"
}

func SolanaRPCURL(network, apiKey string) string {
	return configuredSolanaRPCURL(network, apiKey)
}

func SolanaRPCFallbackURL(network string) string {
	if isSolanaMainnet(network) {
		if value := strings.TrimSpace(os.Getenv("SOLANA_RPC_FALLBACK_URL")); value != "" {
			return value
		}
		return "https://api.mainnet-beta.solana.com"
	}
	if value := strings.TrimSpace(os.Getenv("SOLANA_DEVNET_RPC_FALLBACK_URL")); value != "" {
		return value
	}
	return "https://api.devnet.solana.com"
}

func isSolanaMainnet(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "solana-mainnet", "mainnet", "mainnet-beta":
		return true
	default:
		return false
	}
}

func uniqueRPCURLs(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *SolanaRPC) CacheKey(network, method string, params any) string {
	b, _ := json.Marshal(params)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%s:solana:%s:rpc:%s:%s", s.KeyPrefix, network, method, hex.EncodeToString(h[:]))
}

func TTLFor(method string, params any) time.Duration {
	switch method {
	case "getTokenSupply":
		return time.Minute
	case "getTokenLargestAccounts":
		return 5 * time.Minute
	case "getTransaction":
		return 24 * time.Hour
	case "getSignaturesForAddress":
		return time.Minute
	case "getAccountInfo":
		return 30 * time.Second
	default:
		return 2 * time.Minute
	}
}

func (s *SolanaRPC) Call(ctx context.Context, network, method string, params any, target any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = TTLFor(method, params)
	}
	key := s.CacheKey(network, method, params)
	if ok, err := s.Cache.GetJSON(ctx, key, target); err == nil && ok {
		return nil
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return err
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	for _, endpoint := range uniqueRPCURLs(s.URL(network), SolanaRPCFallbackURL(network)) {
		if err := callSolanaRPC(ctx, client, endpoint, body, target); err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		_ = s.Cache.SetJSON(ctx, key, target, ttl)
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("solana rpc endpoint unavailable")
	}
	return lastErr
}

func callSolanaRPC(ctx context.Context, client *http.Client, endpoint string, body []byte, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rpc status %d", resp.StatusCode)
	}
	var env solanaRPCEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}
	if env.Error != nil {
		return fmt.Errorf("rpc error: %s", env.Error.Message)
	}
	if len(env.Result) == 0 || string(env.Result) == "null" {
		return fmt.Errorf("rpc result unavailable")
	}
	if err := json.Unmarshal(env.Result, target); err != nil {
		return err
	}
	return nil
}
