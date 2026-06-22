package services

import (
	"os"
	"strings"
)

func init() {
	current := strings.TrimSpace(os.Getenv("SOLANA_RPC_URL"))
	if current == "" {
		if value := firstSecurityRadarEnv("ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"); value != "" {
			current = value
		} else if key := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")); key != "" {
			current = "https://solana-mainnet.g.alchemy.com/v2/" + key
		} else {
			current = "https://api.mainnet-beta.solana.com"
		}
	}

	current = normalizeProductionSolanaRPCURL(
		current,
		os.Getenv("APP_ENV"),
		os.Getenv("ALCHEMY_SOLANA_RPC_URL"),
		os.Getenv("ALCHEMY_API_KEY"),
	)
	_ = os.Setenv("SOLANA_RPC_URL", current)
}

func normalizeProductionSolanaRPCURL(rpcURL, appEnv, alchemyURL, alchemyKey string) string {
	rpcURL = strings.TrimSpace(rpcURL)
	if !strings.EqualFold(strings.TrimSpace(appEnv), "production") || !isSolanaDevnetRPCURL(rpcURL) {
		return rpcURL
	}

	alchemyURL = strings.TrimSpace(alchemyURL)
	if alchemyURL != "" && !isSolanaDevnetRPCURL(alchemyURL) {
		return alchemyURL
	}
	if alchemyKey = strings.TrimSpace(alchemyKey); alchemyKey != "" {
		return "https://solana-mainnet.g.alchemy.com/v2/" + alchemyKey
	}
	return "https://api.mainnet-beta.solana.com"
}

func isSolanaDevnetRPCURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "devnet") || strings.Contains(value, "api.devnet.solana.com")
}
