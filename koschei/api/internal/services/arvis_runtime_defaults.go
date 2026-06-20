package services

import (
	"os"
	"strings"
)

func init() {
	if strings.TrimSpace(os.Getenv("ARVIS_HEARTBEAT_SECONDS")) == "" {
		if hasConfiguredArvisRPCProvider() {
			_ = os.Setenv("ARVIS_HEARTBEAT_SECONDS", "20")
		} else {
			_ = os.Setenv("ARVIS_HEARTBEAT_SECONDS", "60")
		}
	}
	if strings.TrimSpace(os.Getenv("ARVIS_STREAM_VERDICT_SECONDS")) == "" {
		if hasConfiguredArvisRPCProvider() {
			_ = os.Setenv("ARVIS_STREAM_VERDICT_SECONDS", "12")
		} else {
			_ = os.Setenv("ARVIS_STREAM_VERDICT_SECONDS", "30")
		}
	}
}

func hasConfiguredArvisRPCProvider() bool {
	for _, key := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		if key == "SOLANA_RPC_URL" && strings.Contains(strings.ToLower(value), "api.mainnet-beta.solana.com") {
			continue
		}
		return true
	}
	return false
}
