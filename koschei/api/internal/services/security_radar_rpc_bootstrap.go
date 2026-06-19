package services

import (
	"os"
	"strings"
)

func init() {
	if strings.TrimSpace(os.Getenv("SOLANA_RPC_URL")) != "" {
		return
	}
	if value := firstSecurityRadarEnv("ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"); value != "" {
		_ = os.Setenv("SOLANA_RPC_URL", value)
		return
	}
	if key := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")); key != "" {
		_ = os.Setenv("SOLANA_RPC_URL", "https://solana-mainnet.g.alchemy.com/v2/"+key)
		return
	}
	_ = os.Setenv("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com")
}
