package services

import (
	"os"
	"strings"

	"koschei/api/internal/web3"
)

// resolvedSolanaRPCURL preserves an explicitly supplied endpoint and otherwise
// follows Koschei's canonical mainnet provider resolution order. Guard callers
// must not fail merely because SOLANA_RPC_URL is empty while a supported
// provider-specific endpoint or Alchemy key is configured.
func resolvedSolanaRPCURL(raw string) string {
	if value := strings.TrimSpace(raw); value != "" {
		return value
	}
	return web3.SolanaRPCURL("solana-mainnet", strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")))
}
