package handlers

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/web3"
)

var (
	tokenServiceMu     sync.Mutex
	globalTokenService *web3.TokenService
)

func (h *Handler) tokenService() *web3.TokenService {
	if globalTokenService != nil {
		return globalTokenService
	}
	tokenServiceMu.Lock()
	defer tokenServiceMu.Unlock()
	if globalTokenService != nil {
		return globalTokenService
	}
	providers := configuredRPCProviders()
	globalTokenService = web3.NewTokenService(web3.NewRPCManager(&http.Client{Timeout: 12 * time.Second}, providers), web3.NewSmartCache(web3.NewMemoryCache()))
	return globalTokenService
}

func configuredRPCProviders() []web3.RPCProviderConfig {
	providers := []web3.RPCProviderConfig{}
	add := func(name, url string, priority int) {
		url = strings.TrimSpace(url)
		if url != "" {
			providers = append(providers, web3.RPCProviderConfig{Name: name, URL: url, Priority: priority, Timeout: 8 * time.Second, Cooldown: time.Minute, MaxFailures: 5})
		}
	}
	alchemyKey := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY"))
	alchemyURL := strings.TrimSpace(os.Getenv("SOLANA_ALCHEMY_RPC_URL"))
	if alchemyURL == "" && alchemyKey != "" {
		alchemyURL = solanaRPCURL("solana-mainnet", alchemyKey)
	}
	add("alchemy", alchemyURL, 10)
	add("helius", os.Getenv("SOLANA_HELIUS_RPC_URL"), 20)
	add("quicknode", os.Getenv("SOLANA_QUICKNODE_RPC_URL"), 30)
	add("custom", os.Getenv("SOLANA_RPC_URL"), 40)
	if len(providers) == 0 {
		add("public", "https://api.mainnet-beta.solana.com", 100)
	}
	return providers
}
